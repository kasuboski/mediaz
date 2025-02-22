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
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// rejectReleaseFunc returns a function that returns true if the given release should be rejected
func rejectReleaseFunc(ctx context.Context, det *model.MovieMetadata, profile storage.QualityProfile, protocolsAvailable map[string]struct{}) func(*prowlarr.ReleaseResource) bool {
	log := logger.FromCtx(ctx)

	return func(r *prowlarr.ReleaseResource) bool {
		if r.Title != nil {
			releaseTitle := strings.TrimSpace(r.Title.MustGet())
			if !strings.HasPrefix(releaseTitle, det.Title) {
				return true
			}
		}

		if r.Protocol != nil {
			// reject if we don't have a download client for it
			if _, has := protocolsAvailable[string(*r.Protocol)]; !has {
				return true
			}
		}
		// bytes to megabytes
		sizeMB := *r.Size >> 20

		// items are assumed to be sorted quality so the highest media quality available is selected
		for _, quality := range profile.Qualities {
			metQuality := MeetsQualitySize(quality, uint64(sizeMB), uint64(det.Runtime))

			if metQuality {
				log.Debugw("accepting release", "release", r.Title, "metQuality", metQuality, "size", r.Size, "runtime", det.Runtime)
				return false
			}

			// try again with the next item
			log.Debugw("rejecting release", "release", r.Title, "metQuality", metQuality, "size", r.Size, "runtime", det.Runtime)
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

const filePattern = `^(?P<title>(?:\w|\s|')+)(?:\s*[(\[]?(?P<year>\d{4})[)\]]?)?(?:\s*\{(?P<edition>[^}]+)\})?(?:\s*(?P<customformat>(?:\[[^\]]+\])*))?(?:[._-]*(?P<releasegroup>[^._\-\s][^._-]*))?$`

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
		// remove quality matches from filename
		for _, m := range qMatches {
			prepdFilename = strings.Replace(prepdFilename, m, "", 1)
		}
	}

	if strings.Contains(prepdFilename, "3d") {
		info := "3D"
		result.Mediainfo3D = &info
		prepdFilename = strings.Replace(prepdFilename, "3d", "", 1)
	}

	// Find matches in the filename
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
			caser := cases.Title(language.English)
			result.Title = strings.TrimSpace(caser.String(value))
		case "year":
			year, err := strconv.Atoi(value)
			if err == nil {
				result.Year = &year
			}
		case "edition":
			// Remove the curly braces
			edition := strings.Trim(value, "{}")
			edition = strings.Replace(edition, "edition-", "", 1)
			result.Edition = &edition
		case "customformat":
			// Remove the square brackets
			customformat := strings.Trim(value, "[]")
			result.Customformat = &customformat
		case "releasegroup":
			result.Releasegroup = &value
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
			case strings.Contains(format, "HDR") || strings.Contains(format, "DV"):
				result.MediainfoDynamicrange = &format
			case strings.Contains(format, "DTS") || strings.Contains(format, "DD") || strings.Contains(format, "Atmos") || strings.Contains(format, "5.1") || strings.Contains(format, "7.1"):
				result.MediainfoAudio = &format
			case strings.Contains(format, "x264") || strings.Contains(format, "x265") || strings.Contains(format, "XviD") || strings.Contains(format, "H.264") || strings.Contains(format, "H264") || strings.Contains(format, "RHS"):
				result.MediainfoVideo = &format
			}
		}
	}

	return result, true
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
		if strings.Contains(name, strings.ToLower(med)) {
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

// determineSeparator tries to determine the separator between the various parts of the filename
// It assumes it is one of `.`, `_`, `-`, ` `
// It decides based on which on is most present in the filename
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
