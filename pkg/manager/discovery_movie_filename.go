package manager

import (
	"path/filepath"
	"regexp"
	"strings"
)

var (
	movieAuxLabel       = regexp.MustCompile(`(?i)(?:^|[ ._\-])(?:interview|trailer|featurette|deleted scenes?|behind the scenes?|bonus|extras?)$`)
	movieDashPartSuffix = regexp.MustCompile(`-[12]$`)
)

// movieDiscoveryFilename returns the title-like filename identity and whether
// it should contribute evidence. Auxiliary labels require a matching feature
// sibling; title digits are retained except for corroborated -1/-2 split files.
func movieDiscoveryFilename(folder string, raw string, siblings []string) (name string, keep bool) {
	base := filepath.Base(raw)
	name = strings.TrimSpace(strings.TrimSuffix(base, filepath.Ext(base)))
	if name == "" {
		return name, false
	}
	if movieAuxLabel.MatchString(normalizeSeparators(name)) && !movieTitleAgreesWithFolder(name, folder) && movieHasFolderFeature(folder, siblings) {
		return name, false
	}
	if loc := movieDashPartSuffix.FindStringIndex(name); loc != nil {
		prefix := strings.TrimSpace(name[:loc[0]])
		if movieHasMultipartCorroboration(prefix, siblings) {
			name = prefix
		}
	}
	return name, name != ""
}

func movieHasFolderFeature(folder string, siblings []string) bool {
	for _, sibling := range siblings {
		if movieTitleAgreesWithFolder(strings.TrimSuffix(filepath.Base(sibling), filepath.Ext(sibling)), folder) {
			return true
		}
	}
	return false
}

func movieTitleAgreesWithFolder(name, folder string) bool {
	name = discoveryID.ReplaceAllString(name, " ")
	folder = discoveryID.ReplaceAllString(filepath.Base(filepath.Clean(folder)), " ")
	title, _ := discoverySearchTerm(name)
	folderTitle, _ := discoverySearchTerm(folder)
	a, b := normalizeReleaseIdentity(title), normalizeReleaseIdentity(folderTitle)
	return a != "" && a == b
}

func movieHasMultipartCorroboration(prefix string, siblings []string) bool {
	wanted := normalizeReleaseIdentity(prefix)
	unsuffixed, first, second := false, false, false
	for _, raw := range siblings {
		base := filepath.Base(raw)
		n := strings.TrimSpace(strings.TrimSuffix(base, filepath.Ext(base)))
		if normalizeReleaseIdentity(n) == wanted {
			unsuffixed = true
		}
		if m := movieDashPartSuffix.FindStringSubmatch(n); m != nil && normalizeReleaseIdentity(movieDashPartSuffix.ReplaceAllString(n, "")) == wanted {
			if strings.HasSuffix(n, "-1") {
				first = true
			}
			if strings.HasSuffix(n, "-2") {
				second = true
			}
		}
	}
	return unsuffixed && first && second
}
