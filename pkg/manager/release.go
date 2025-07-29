package manager

import (
	"cmp"
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/kasuboski/mediaz/pkg/logger"
	"github.com/kasuboski/mediaz/pkg/prowlarr"
	"github.com/kasuboski/mediaz/pkg/storage"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var (
	seasonPackPattern    = regexp.MustCompile(`(?i)(?:S(?:eason)?[\s._-]?\d{1,2}|[\s._]Complete[\s._])`)
	episodePattern       = regexp.MustCompile(`(?i)\b(S\d{1,2}E\d{2,})|\b(\d{1,2}x\d{2,})`)
	episodeNumberPattern = regexp.MustCompile(`(?i)S?(\d{1,2})(?:E|x)(\d{1,2})`)
	seasonNumberPattern  = regexp.MustCompile(`(?i)(?:S(?:eason)?[\s._-]?(\d{1,2}))`)

	// pathToSearchTerm regex patterns
	releaseGroupRegex   = regexp.MustCompile(`-[A-Z0-9]+\[[^\]]+\]`)
	yearRegex           = regexp.MustCompile(`\s*[\(\[\{]?(19|20)\d{2}[\)\]\}]?(?:\s|$)`)
	qualityRegex        = regexp.MustCompile(`(?i)\b(720p|1080p|4k|2160p|x264|h264|hevc|web-dl|bluray|dvdrip|cam|ts|tc)\b`)
	codecRegex          = regexp.MustCompile(`(?i)\b(h264|ac3|aac|dts|dd5\.1)\b`)
	emptyBracketsRegex  = regexp.MustCompile(`\s*[\[\(\{][\]\)\}]\s*`)
	multipleSpacesRegex = regexp.MustCompile(`\s+`)
)

func RejectMovieReleaseFunc(ctx context.Context, title string, runtime int32, profile storage.QualityProfile, protocolsAvailable map[string]struct{}) func(*prowlarr.ReleaseResource) bool {
	return func(r *prowlarr.ReleaseResource) bool {
		if r == nil {
			return true
		}

		if r.Title != nil {
			releaseTitle := strings.TrimSpace(r.Title.MustGet())
			if !strings.HasPrefix(releaseTitle, title) {
				return true
			}
		}

		return rejectReleaseFunc(ctx, runtime, profile, protocolsAvailable)(r)
	}
}

func RejectSeasonReleaseFunc(ctx context.Context, seriesTitle string, seasonNumber, runtime int32, profile storage.QualityProfile, protocolsAvailable map[string]struct{}) func(*prowlarr.ReleaseResource) bool {
	return func(r *prowlarr.ReleaseResource) bool {
		if rejectSeasonReleaseFunc(seriesTitle, seasonNumber, r) {
			return true
		}
		return rejectReleaseFunc(ctx, runtime, profile, protocolsAvailable)(r)
	}
}

func rejectSeasonReleaseFunc(seriesTitle string, seasonNumber int32, r *prowlarr.ReleaseResource) bool {
	if r == nil {
		return true
	}

	foundTitle, err := r.Title.Get()
	if err != nil {
		return true
	}

	if !seasonPackPattern.MatchString(foundTitle) {
		return true
	}

	// we dont want individual episodes here
	if episodePattern.MatchString(foundTitle) {
		return true
	}

	normalizedSeriesTitle := strings.ToLower(seriesTitle)
	normalizedReleaseTitle := strings.ToLower(foundTitle)

	if !strings.Contains(normalizedReleaseTitle, normalizedSeriesTitle) {
		return true
	}

	matches := seasonNumberPattern.FindStringSubmatch(normalizedReleaseTitle)
	for _, m := range matches {
		if strings.Contains(m, fmt.Sprintf("%d", seasonNumber)) {
			return false
		}
	}

	return true
}

func RejectEpisodeReleaseFunc(ctx context.Context, seriesTitle string, seasonNumber, episodeNumber, runtime int32, profile storage.QualityProfile, protocolsAvailable map[string]struct{}) func(*prowlarr.ReleaseResource) bool {
	return func(r *prowlarr.ReleaseResource) bool {
		if rejectEpisodeReleaseFunc(seriesTitle, seasonNumber, episodeNumber, r) {
			return true
		}

		return rejectReleaseFunc(ctx, runtime, profile, protocolsAvailable)(r)
	}
}

func rejectEpisodeReleaseFunc(seriesTitle string, seasonNumber, episodeNumber int32, r *prowlarr.ReleaseResource) bool {
	if r == nil {
		return true
	}

	foundTitle, err := r.Title.Get()
	if err != nil {
		return true
	}

	normalizedSeriesTitle := strings.ToLower(seriesTitle)
	normalizedReleaseTitle := strings.ToLower(foundTitle)

	if !strings.Contains(normalizedReleaseTitle, normalizedSeriesTitle) {
		return true
	}

	matches := episodeNumberPattern.FindStringSubmatch(normalizedReleaseTitle)
	if len(matches) != 3 {
		return true
	}

	season, err := strconv.ParseInt(matches[1], 10, 32)
	if err != nil {
		return true
	}

	episode, err := strconv.ParseInt(matches[2], 10, 32)
	if err != nil {
		return true
	}

	match := !(int32(season) == seasonNumber && int32(episode) == episodeNumber)
	return match
}

// rejectReleaseFunc returns a function that returns true if the given release should be rejected
func rejectReleaseFunc(ctx context.Context, runtime int32, profile storage.QualityProfile, protocolsAvailable map[string]struct{}) func(*prowlarr.ReleaseResource) bool {
	log := logger.FromCtx(ctx)

	return func(r *prowlarr.ReleaseResource) bool {
		if r == nil {
			return true
		}

		if r.Protocol != nil {
			// reject if we don't have a download client for it
			if _, has := protocolsAvailable[string(*r.Protocol)]; !has {
				return true
			}
		}

		if r.FileName.IsSpecified() {
			_, parsed := parseReleaseFilename(r.FileName.MustGet())
			if !parsed {
				return true
			}
		}

		// bytes to megabytes
		sizeMB := *r.Size >> 20

		// items are assumed to be sorted quality so the highest media quality available is selected
		for _, quality := range profile.Qualities {

			metQuality := MeetsQualitySize(quality, uint64(sizeMB), uint64(runtime))
			if metQuality {
				log.Debugw("accepting release", "release", r.Title, "metQuality", metQuality, "size", r.Size, "runtime", runtime)
				return false
			}

			// try again with the next item
			log.Debugw("rejecting release", "release", r.Title, "metQuality", metQuality, "size", r.Size, "runtime", runtime)
		}

		return true
	}
}

// sortReleaseFunc returns a function that sorts releases by their number of seeders currently
func sortReleaseFunc() func(*prowlarr.ReleaseResource, *prowlarr.ReleaseResource) int {
	return func(r1 *prowlarr.ReleaseResource, r2 *prowlarr.ReleaseResource) int {
		return cmp.Compare(nullableDefault(r1.Seeders), nullableDefault(r2.Seeders))
	}
}

// ParsedReleaseFile represents a parsed movie release filename with extracted metadata
type ParsedReleaseFile struct {
	Filename              string  `json:"filename"`
	Title                 string  `json:"title"`
	Year                  *int    `json:"year"`
	Edition               *string `json:"edition"`
	Customformat          *string `json:"customformat"`
	Quality               *string `json:"quality"`
	Mediainfo3D           *string `json:"mediainfo_3d"`
	MediainfoDynamicrange *string `json:"mediainfo_dynamicrange"`
	MediainfoAudio        *string `json:"mediainfo_audio"`
	MediainfoVideo        *string `json:"mediainfo_video"`
	Releasegroup          *string `json:"releasegroup"`
}

const filePattern = `^(?P<title>(?:\w|\s|')+)(?:\s*[(\[]?(?P<year>\d{4})[)\]]?)?(?:\s*\{(?P<edition>[^}]+)\})?(?:\s*(?P<customformat>(?:\[[^\]]+\])*))?.*$`

var releaseFileRegex = regexp.MustCompile(filePattern)

// parseReleaseFilename parses a release filename into a ParsedReleaseFile struct
// if the filename does not match ok will be false
func parseReleaseFilename(filename string) (ParsedReleaseFile, bool) {
	// Create result struct with filename
	result := ParsedReleaseFile{
		Filename: filename,
	}

	sep := determineSeparator(filename)
	prepdFilename := strings.ToLower(strings.ReplaceAll(filename, sep, " "))

	quality, qMatches := findQuality(filename)
	if len(qMatches) > 0 {
		result.Quality = &quality
		prepdFilename = removeFromName(prepdFilename, qMatches...)
	}

	dynamicRange := findDynamicRange(filename)
	if len(dynamicRange) > 0 {
		dRange := strings.Join(dynamicRange, " ")
		result.MediainfoDynamicrange = &dRange
		prepdFilename = removeFromName(prepdFilename, dynamicRange...)
	}

	audioInfo := findAudioInfo(filename)
	if len(audioInfo) > 0 {
		audio := strings.Join(audioInfo, " ")
		result.MediainfoAudio = &audio
		prepdFilename = removeFromName(prepdFilename, audioInfo...)
	}

	videoInfo := findVideoInfo(filename)
	if len(videoInfo) > 0 {
		video := strings.Join(videoInfo, " ")
		result.MediainfoVideo = &video
		prepdFilename = removeFromName(prepdFilename, videoInfo...)
	}

	if strings.Contains(prepdFilename, "3d") {
		info := "3D"
		result.Mediainfo3D = &info
		prepdFilename = removeFromName(prepdFilename, "3d")
	}

	stream := findStreamingService(prepdFilename)
	if stream != "" {
		prepdFilename = removeFromName(prepdFilename, stream)
	}

	group, matched := findReleaseGroup(prepdFilename)
	if group != "" {
		group = titleCase(group)
		result.Releasegroup = &group
		prepdFilename = removeFromName(prepdFilename, matched)
	}

	matches := releaseFileRegex.FindStringSubmatch(prepdFilename)
	if len(matches) == 0 {
		return result, false
	}

	// Get named capture group indices
	groupNames := releaseFileRegex.SubexpNames()
	for i, name := range groupNames {
		if i == 0 { // Skip the full match
			continue
		}

		// Get the matched value for this group
		if i >= len(matches) {
			break
		}
		value := matches[i]
		if value == "" {
			continue
		}

		// Set the appropriate field based on group name
		switch name {
		case "title":
			result.Title = titleCase(value)
		case "year":
			year, err := strconv.Atoi(value)
			if err == nil {
				result.Year = &year
			}
		case "edition":
			// Remove the curly braces
			edition := strings.Trim(value, "{}")
			edition = titleCase(strings.Replace(edition, "edition-", "", 1))
			result.Edition = &edition
		case "customformat":
			// Remove the square brackets
			customFormat := strings.Trim(value, "[]")
			combined := customFormat + stream
			result.Customformat = &combined
		}
	}

	// Parse additional metadata from customformat if present
	if result.Customformat != nil {
		formats := strings.Split(*result.Customformat, "][")
		for _, format := range formats {
			format = strings.TrimSpace(format)

			// Try to identify the type of format
			switch {
			case strings.Contains(format, "IMAX") || strings.Contains(format, "AMZN"):
				result.Customformat = &format
			}
		}
	}

	if result.Customformat == nil && stream != "" {
		result.Customformat = &stream
	}

	return result, true
}

// findVideoInfo looks for all video related info in the filename. It returns all of them
func findVideoInfo(filename string) []string {
	encodings := []string{"x264", "x265", "H.264", "H264"}
	videoStrings := make([]string, 0)
	name := strings.ToLower(filename)
	for _, e := range encodings {
		if strings.Contains(name, strings.ToLower(e)) {
			videoStrings = append(videoStrings, e)
		}
	}
	return videoStrings
}

// findAudioInfo looks for all audio related info in the filename. It returns all of them
func findAudioInfo(filename string) []string {
	// Define a list of audio strings
	formats := []string{"DDPlus", "TrueHD", "DTS-HD", "DTS X", "DDP", "DD", "DTS", "Atmos"}
	channels := []string{"stereo", "2.0", "2_0", "5.1", "5_1", "7.1", "7_1", "7 1"}
	combo := make([]string, 0)

	audioStrings := make([]string, 0)
	name := strings.ToLower(filename)

	for _, f := range formats {
		for _, c := range channels {
			combo = append(combo, f+c)
		}
	}
	foundValue := ""
	for _, c := range combo {
		if strings.Contains(name, strings.ToLower(c)) {
			foundValue = c
			break
		}
	}
	if foundValue != "" {
		normalized := strings.ReplaceAll(strings.ReplaceAll(foundValue, "_", "."), " ", ".")
		audioStrings = append(audioStrings, normalized)
		// if found a combo assume done
		return audioStrings
	}

	foundFormats := findMatchingWords(name, formats)
	audioStrings = append(audioStrings, foundFormats...)
	channelValue := ""
	for _, c := range channels {
		if strings.Contains(name, strings.ToLower(c)) {
			channelValue = c
			break
		}
	}
	if channelValue != "" {
		normalized := strings.ReplaceAll(strings.ReplaceAll(channelValue, "_", "."), " ", ".")
		audioStrings = append(audioStrings, normalized)
	}

	return audioStrings
}

// findDynamicRange parses the filename looking for a dynamic range from a predefined list
func findDynamicRange(filename string) []string {
	name := strings.ToLower(filename)

	found := make([]string, 0)

	if strings.Contains(name, strings.ToLower("DV")) {
		found = append(found, "DV")
	}

	if strings.Contains(name, strings.ToLower("HDR10")) {
		found = append(found, "HDR10")
	} else if strings.Contains(name, strings.ToLower("HDR")) {
		found = append(found, "HDR")
	}

	return found
}

// findQuality parses the filename looking for a quality from a predefined list
func findQuality(filename string) (quality string, matches []string) {
	// Define a list of quality strings
	resolutions := []string{"720", "1080", "2160"}
	media := []string{"Bluray", "BRRip", "HDTV", "WEBDL", "WEBRip", "Remux", "SDTV", "DVD", "WEB", "HD"}
	qualities := make([]string, 0)

	// media-resolution with p
	for _, res := range resolutions {
		for _, med := range media {
			qualities = append(qualities, fmt.Sprintf("%s-%sp", med, res))
		}
	}

	name := strings.ToLower(filename)
	// Normalize WEB-DL to WEBDL
	name = strings.ReplaceAll(name, "web-dl", "webdl")

	for _, quality := range qualities {
		q := strings.ToLower(quality)
		if strings.Contains(name, q) {
			// if we find a quality with both media and resolution, return it
			return quality, []string{q}
		}
	}

	// otherwise check if we can find the media and resolution separately
	foundMedia := ""
	for _, med := range media {
		medLower := strings.ToLower(med)
		// Also check for hyphenated version
		if strings.Contains(name, medLower) || strings.Contains(name, strings.ReplaceAll(medLower, "dl", "-dl")) {
			foundMedia = med
			break
		}
	}

	foundResolution := ""
	for _, res := range resolutions {
		if strings.Contains(name, strings.ToLower(res)) {
			foundResolution = res
			break
		}
	}

	if foundMedia != "" && foundResolution != "" {
		return fmt.Sprintf("%s-%sp", foundMedia, foundResolution), []string{foundMedia, foundResolution}
	}

	if foundMedia != "" && foundResolution == "" {
		return foundMedia, []string{foundMedia}
	}

	if foundMedia == "" && foundResolution != "" {
		return foundResolution + "p", []string{foundResolution}
	}

	return "", nil
}

var regexGroup = regexp.MustCompile(`[-\(]([^-\(\)]+)\)?$`)

func findReleaseGroup(filename string) (group string, matched string) {
	matches := regexGroup.FindStringSubmatch(filename)
	// expected 2 matches. 1 for the full match, 1 for the group
	if len(matches) != 2 {
		return "", ""
	}

	return matches[1], matches[0]

}

func findStreamingService(filename string) string {
	name := strings.ToLower(filename)

	if strings.Contains(name, "netflix") {
		return "NF"
	} else if len(findMatchingWords(name, []string{"amazon", "amzn"})) > 0 {
		return "AMZN"
	} else if strings.Contains(name, "hulu") {
		return "Hulu"
	} else if len(findMatchingWords(name, []string{"disney", "dsnp"})) > 0 {
		return "DSNP"
	}
	return ""
}

// determineSeparator tries to determine the separator between the various parts of the filename
// It assumes it is one of `.`, `_`, `-`, ` `
// It decides based on which one is most present in the filename
func determineSeparator(filename string) string {
	count := 0
	currSep := ""
	for _, sep := range []string{".", "_", "-", " "} {
		if strings.Count(filename, sep) > count {
			count = strings.Count(filename, sep)
			currSep = sep
		}
	}

	return currSep
}

func titleCase(title string) string {
	title = strings.TrimSpace(title)
	return cases.Title(language.English).String(title)
}

// pathToSearchTerm takes a movie path and removes the year if present, preserving alternate titles
func pathToSearchTerm(path string) string {
	if path == "" {
		return ""
	}

	// Replace dots with spaces (for movie.name.format)
	cleaned := strings.ReplaceAll(path, ".", " ")

	// Remove release group indicators (e.g., -EVO[EtHD])
	cleaned = releaseGroupRegex.ReplaceAllString(cleaned, "")

	// Remove years in various formats: (YYYY), {YYYY}, [YYYY], or trailing YYYY
	cleaned = yearRegex.ReplaceAllString(cleaned, " ")

	// Remove common quality indicators
	cleaned = qualityRegex.ReplaceAllString(cleaned, "")

	// Remove codec and audio info
	cleaned = codecRegex.ReplaceAllString(cleaned, "")

	// Remove empty brackets and parentheses
	cleaned = emptyBracketsRegex.ReplaceAllString(cleaned, " ")

	// Clean up multiple spaces
	cleaned = multipleSpacesRegex.ReplaceAllString(cleaned, " ")

	return strings.TrimSpace(cleaned)
}

func removeFromName(filename string, toRemove ...string) string {
	lowerRemove := make([]string, len(toRemove))
	for i, r := range toRemove {
		lowerRemove[i] = regexp.QuoteMeta(strings.ToLower(r))
	}

	rmRegex, err := regexp.Compile(fmt.Sprintf(`(?i)[\[\(]+[^\](]*(?:%s)[^[)]*[\]\)]+`, strings.Join(lowerRemove, "|")))
	if err != nil {
		return filename
	}
	return rmRegex.ReplaceAllLiteralString(filename, "")
}

// findMatchingWords takes a source string and a slice of candidate words
// Returns a slice of words that were found as exact matches
func findMatchingWords(source string, candidates []string) []string {
	// Convert source to lowercase and split into words
	reg := regexp.MustCompile(`\b[\w]+\b`)
	sourceWords := reg.FindAllString(strings.ToLower(source), -1)

	wordMap := make(map[string]bool)
	for _, word := range sourceWords {
		wordMap[word] = true
	}

	// Check each candidate for exact matches
	matches := make([]string, 0)
	for _, candidate := range candidates {
		if wordMap[strings.ToLower(candidate)] {
			matches = append(matches, candidate)
		}
	}

	return matches
}
