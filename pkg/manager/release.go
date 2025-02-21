package manager

import (
	"cmp"
	"context"
	"regexp"
	"strconv"
	"strings"

	"github.com/kasuboski/mediaz/pkg/logger"
	"github.com/kasuboski/mediaz/pkg/prowlarr"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
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

var releaseFileRegex = regexp.MustCompile(`^(?P<title>.*?)(?:\s*[\(\[]?(?P<year>\d{4})[\)\]]?)?\s*(?P<edition>\{[^}]+\})?\s*(?P<customformat>(?:\[[^\]]+\])*)?(?:\s*-\s*(?P<releasegroup>[^-\s][^-]*)?)?(?:\.(?:mkv|mp4|avi|torrent|nzb))?$`)

// parseReleaseFilename parses a release filename into a ParsedReleaseFile struct
// if the filename does not match ok will be false
func parseReleaseFilename(filename string) (ParsedReleaseFile, bool) {
	// Create result struct with filename
	result := ParsedReleaseFile{
		Filename: filename,
	}

	// Find matches in the filename
	matches := releaseFileRegex.FindStringSubmatch(filename)
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
			result.Title = strings.TrimSpace(value)
		case "year":
			year, err := strconv.Atoi(value)
			if err == nil {
				result.Year = &year
			}
		case "edition":
			// Remove the curly braces
			edition := strings.Trim(value, "{}")
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
			case strings.Contains(format, "3D"):
				result.Mediainfo3D = &format
			case strings.Contains(format, "HDR") || strings.Contains(format, "DV"):
				result.MediainfoDynamicrange = &format
			case strings.Contains(format, "DTS") || strings.Contains(format, "DD") || strings.Contains(format, "Atmos") || strings.Contains(format, "5.1") || strings.Contains(format, "7.1"):
				result.MediainfoAudio = &format
			case strings.Contains(format, "x264") || strings.Contains(format, "x265") || strings.Contains(format, "XviD") || strings.Contains(format, "H.264") || strings.Contains(format, "H264") || strings.Contains(format, "RHS"):
				result.MediainfoVideo = &format
			case strings.Contains(format, "1080p") || strings.Contains(format, "720p") || strings.Contains(format, "2160p") || strings.Contains(format, "Bluray") || strings.Contains(format, "BRRip") || strings.Contains(format, "WEB"):
				result.Quality = &format
			}
		}
	}

	return result, true
}
