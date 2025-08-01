package manager

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/kasuboski/mediaz/pkg/prowlarr"
	"github.com/oapi-codegen/nullable"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testCases struct {
	Testcases []ParsedReleaseFile `json:"testcases"`
}

func TestParseReleaseFilename(t *testing.T) {
	b, err := os.ReadFile("testing/parse-moviefiles.json")
	require.NoError(t, err)

	var cases testCases
	err = json.Unmarshal(b, &cases)
	require.NoError(t, err)

	for _, tc := range cases.Testcases {
		t.Run(tc.Filename, func(t *testing.T) {
			parsed, ok := parseReleaseFilename(tc.Filename)
			require.True(t, ok, "failed to parse filename")

			assert.Equal(t, tc.Filename, parsed.Filename)
			// don't worry about title and year for now; maybe we similarity search or exact match from desired
			// assert.Equal(t, tc.Title, parsed.Title)
			// equalValuesPrettyPrint(t, tc.Year, parsed.Year)
			equalValuesPrettyPrint(t, tc.Edition, parsed.Edition)
			equalValuesPrettyPrint(t, tc.Customformat, parsed.Customformat)
			equalValuesPrettyPrint(t, tc.Quality, parsed.Quality)
			equalValuesPrettyPrint(t, tc.Mediainfo3D, parsed.Mediainfo3D)
			assertArrayString(t, tc.MediainfoDynamicrange, parsed.MediainfoDynamicrange)
			assertArrayString(t, tc.MediainfoAudio, parsed.MediainfoAudio)
			equalValuesPrettyPrint(t, tc.MediainfoVideo, parsed.MediainfoVideo)
			// don't actually really care atm
			// equalValuesPrettyPrint(t, tc.Releasegroup, parsed.Releasegroup)
		})
	}
}

func TestFindQuality(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     string
	}{
		{
			name:     "full quality string with multiple formats",
			filename: "The Movie Title (2010) {edition-Ultimate Extended Edition} [IMAX HYBRID][Bluray-1080p Proper][3D][DV HDR10][DTS 5.1][x264]-EVOLVE",
			want:     "Bluray-1080p",
		},
		{
			name:     "simple 720p quality",
			filename: "Brothers 2024 720p [broski]",
			want:     "720p",
		},
		{
			name:     "2160p with multiple formats",
			filename: "Step Brothers 2008 2160p UNRATED Bluray x265 DDP Atmos DTS KiNGDOM",
			want:     "Bluray-2160p",
		},
		{
			name:     "BRRip quality",
			filename: "The-Brothers-Karamazov-1969-(Dostoevsky-Mini-Series)-1080p-BRRip-x264-Classics",
			want:     "BRRip-1080p",
		},
		{
			name:     "WEB quality",
			filename: "Brothers 2024 1080p AMZN WEB DLip ExKinoRay",
			want:     "WEB-1080p",
		}, // TODO: We want to match web-dl, but return webdl
		// {
		// 	name:     "WEB-DL quality",
		// 	filename: "The.Brothers.Karamazov.1969.(Dostoevsky.Mini.Series).1080p.WEB-DL.x264.Classics",
		// 	want:     "WEBDL-1080p",
		// },
		// Edge cases
		{
			name:     "empty string",
			filename: "",
			want:     "",
		},
		{
			name:     "no quality string",
			filename: "Just A Movie Title",
			want:     "",
		},
		{
			name:     "mixed case quality",
			filename: "Movie (2024) bLuRaY 1080P",
			want:     "Bluray-1080p",
		},
		{
			name:     "HD only",
			filename: "Movie (2024) HD",
			want:     "HD",
		},
		{
			name:     "similar but invalid quality",
			filename: "Movie (2024) 1081",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// not testing what it matched
			got, _ := findQuality(tt.filename)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDetermineSeparator(t *testing.T) {
	tests := []struct {
		name string
		args string
		want string
	}{
		{
			name: "dash",
			args: "movie-name-2021",
			want: "-",
		},
		{
			name: "underscore",
			args: "movie_name_2021",
			want: "_",
		},
		{
			name: "dot",
			args: "movie.name.2021",
			want: ".",
		},
		{
			name: "space",
			args: "movie name 2021",
			want: " ",
		},
		{
			name: "mixed",
			args: "movie_name_2021-RAR.mkv",
			want: "_",
		},
		{
			name: "full filename",
			args: "The Movie Title (2010) {edition-Ultimate Extended Edition} [IMAX HYBRID][Bluray-1080p Proper][3D][DV HDR10][DTS 5.1][x264]-EVOLVE",
			want: " ",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := determineSeparator(tt.args); got != tt.want {
				t.Errorf("determineSeparator() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRemoveFromName(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		toRemove []string
		want     string
	}{
		{
			name:     "remove year",
			filename: "The Movie Title (2010) {edition-Ultimate Extended Edition} [IMAX HYBRID][Bluray-1080p Proper][3D][DV HDR10][DTS 5.1][x264]-EVOLVE",
			toRemove: []string{"2010"},
			want:     "The Movie Title  {edition-Ultimate Extended Edition} [IMAX HYBRID][Bluray-1080p Proper][3D][DV HDR10][DTS 5.1][x264]-EVOLVE",
		},
		{
			name:     "remove quality",
			filename: "The Movie Title (2010) {edition-Ultimate Extended Edition} [IMAX HYBRID][Bluray-1080p Proper][3D][DV HDR10][DTS 5.1][x264]-EVOLVE",
			toRemove: []string{"Bluray-1080p"},
			want:     "The Movie Title (2010) {edition-Ultimate Extended Edition} [IMAX HYBRID][3D][DV HDR10][DTS 5.1][x264]-EVOLVE",
		},
		{
			name:     "remove audio info",
			filename: "The Movie Title (2010) {edition-Ultimate Extended Edition} [IMAX HYBRID][Bluray-1080p Proper][3D][DV HDR10][DTS 5.1][x264]-EVOLVE",
			toRemove: []string{"DTS 5.1"},
			want:     "The Movie Title (2010) {edition-Ultimate Extended Edition} [IMAX HYBRID][Bluray-1080p Proper][3D][DV HDR10][x264]-EVOLVE",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := removeFromName(tt.filename, tt.toRemove...)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPathToSearchTerm(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "movie with year",
			path: "Zoolander (2001)",
			want: "Zoolander",
		},
		{
			name: "movie without year",
			path: "Zoolander",
			want: "Zoolander",
		},
		{
			name: "movie with alternate title",
			path: "Zoolander (Blue Steel)",
			want: "Zoolander (Blue Steel)",
		},
		{
			name: "movie with year and alternate title",
			path: "Zoolander (Blue Steel) (2001)",
			want: "Zoolander (Blue Steel)",
		},
		{
			name: "empty string",
			path: "",
			want: "",
		},
		// Failing test cases from the issue
		{
			name: "year in curly braces",
			path: "Apocalypse Now {1979}",
			want: "Apocalypse Now",
		},
		{
			name: "dotted filename with year and quality tags",
			path: "Columbus.2017.1080p.WEB-DL.H264.AC3-EVO[EtHD]",
			want: "Columbus",
		},
		{
			name: "year at end without parentheses",
			path: "Der Untergang - Downfall 2004",
			want: "Der Untergang - Downfall",
		},
		{
			name: "year in parentheses with quality",
			path: "Hugo (2011) 720p",
			want: "Hugo",
		},
		{
			name: "year in parentheses with quality in brackets",
			path: "Hunt for the Wilderpeople (2016) [1080p]",
			want: "Hunt for the Wilderpeople",
		},
		{
			name: "year in square brackets",
			path: "Guardians of the Galaxy [2014] 1080p",
			want: "Guardians of the Galaxy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pathToSearchTerm(tt.path)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFindMatchingWords(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		words    []string
		expected []string
	}{
		{
			name:     "basic match",
			source:   "This is a test string",
			words:    []string{"is", "test"},
			expected: []string{"is", "test"},
		},
		{
			name:     "no match",
			source:   "This is a test string",
			words:    []string{"not", "present"},
			expected: []string{},
		},
		{
			name:     "mixed case match",
			source:   "This IS a Test string",
			words:    []string{"is", "test"},
			expected: []string{"is", "test"},
		},
		{
			name:     "prefix match",
			source:   "I have DDP 5.1 audio",
			words:    []string{"ddp", "dd"},
			expected: []string{"ddp"},
		},
		{
			name:     "match past word",
			source:   "I have DDP5.1 audio",
			words:    []string{"ddp", "dts"},
			expected: []string{},
		},
		{
			name:     "combo match doesn't work",
			source:   "The.Menendez.Brothers.2024.1080p.WEBRip.1400MB.DD5.1.x264-Galaxy",
			words:    []string{"DD5.1", "Atmos5_1", "DST5_1"},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, findMatchingWords(tt.source, tt.words))
		})
	}
}

func TestRejectSeasonReleaseFunc(t *testing.T) {
	tests := []struct {
		name         string
		seriesTitle  string
		seasonNumber int32
		release      *prowlarr.ReleaseResource
		want         bool
	}{
		{
			name:         "nil release",
			seriesTitle:  "test series",
			seasonNumber: 1,
			release:      nil,
			want:         true,
		},
		{
			name:         "valid season pack",
			seriesTitle:  "ShowName",
			seasonNumber: 3,
			release: &prowlarr.ReleaseResource{
				Title: nullable.NewNullableWithValue("ShowName.S03.1080p.WEB-DL.HEVC.x265"),
			},
			want: false,
		},
		{
			name:         "invalid season pack",
			seriesTitle:  "ShowName",
			seasonNumber: 3,
			release: &prowlarr.ReleaseResource{
				Title: nullable.NewNullableWithValue("Show.Name.S03E01.1080p.WEB-DL"),
			},
			want: true,
		},
		{
			name:         "valid season pack with 'season' in name",
			seriesTitle:  "ShowName",
			seasonNumber: 3,
			release: &prowlarr.ReleaseResource{
				Title: nullable.NewNullableWithValue("ShowName.season.3.1080p.WEB-DL.HEVC.x265"),
			},
		},
		{
			name:         "valid release with group",
			seriesTitle:  "ShowName",
			seasonNumber: 7,
			release: &prowlarr.ReleaseResource{
				Title: nullable.NewNullableWithValue("ShowName.S07.1080p.WEB-DL.AAC2.0.x264-Group"),
			},
			want: false,
		},
		{
			name:         "double digit sesaon number",
			seriesTitle:  "ShowName",
			seasonNumber: 10,
			release: &prowlarr.ReleaseResource{
				Title: nullable.NewNullableWithValue("ShowName.S10.1080p.WEB-DL.AAC2.0.x264-Group"),
			},
		},
		{
			name:         "underscores",
			seriesTitle:  "ShowName",
			seasonNumber: 3,
			release: &prowlarr.ReleaseResource{
				Title: nullable.NewNullableWithValue("ShowName_Season_03_Complete_720p.HDTV"),
			},
		},
		{
			name:         "unrelated show",
			seriesTitle:  "ShowName",
			seasonNumber: 3,
			release: &prowlarr.ReleaseResource{
				Title: nullable.NewNullableWithValue("yoyo.S10.1080p.WEB-DL.AAC2.0.x264-Group"),
			},
			want: true,
		},
		{
			name:         "unrelated release",
			seriesTitle:  "ShowName",
			seasonNumber: 3,
			release: &prowlarr.ReleaseResource{
				Title: nullable.NewNullableWithValue("asdfadfadsfad"),
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rejectSeasonReleaseFunc(tt.seriesTitle, tt.seasonNumber, tt.release)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRejectEpisodeReleaseFunc(t *testing.T) {
	tests := []struct {
		name          string
		episodeTitle  string
		seasonNumber  int32
		episodeNumber int32
		release       *prowlarr.ReleaseResource
		want          bool
	}{
		{
			name:          "nil release",
			episodeTitle:  "ShowName",
			seasonNumber:  1,
			episodeNumber: 1,
			release:       nil,
			want:          true,
		},
		{
			name:          "standard episode format",
			episodeTitle:  "ShowName",
			seasonNumber:  1,
			episodeNumber: 2,
			release: &prowlarr.ReleaseResource{
				Title: nullable.NewNullableWithValue("ShowName.S01E02.1080p.WEB-DL.AAC2.0.x264-GROUP"),
			},
			want: false,
		},
		{
			name:          "alternate episode format",
			episodeTitle:  "ShowName",
			seasonNumber:  1,
			episodeNumber: 2,
			release: &prowlarr.ReleaseResource{
				Title: nullable.NewNullableWithValue("ShowName.1x02.1080p.WEB-DL.AAC2.0.x264-GROUP"),
			},
			want: false,
		},
		{
			name:          "wrong season",
			episodeTitle:  "ShowName",
			seasonNumber:  1,
			episodeNumber: 2,
			release: &prowlarr.ReleaseResource{
				Title: nullable.NewNullableWithValue("ShowName.S02E02.1080p.WEB-DL.AAC2.0.x264-GROUP"),
			},
			want: true,
		},
		{
			name:          "wrong episode",
			episodeTitle:  "ShowName",
			seasonNumber:  1,
			episodeNumber: 2,
			release: &prowlarr.ReleaseResource{
				Title: nullable.NewNullableWithValue("ShowName.S01E03.1080p.WEB-DL.AAC2.0.x264-GROUP"),
			},
			want: true,
		},
		{
			name:          "double digit season and episode",
			episodeTitle:  "ShowName",
			seasonNumber:  10,
			episodeNumber: 12,
			release: &prowlarr.ReleaseResource{
				Title: nullable.NewNullableWithValue("ShowName.S10E12.1080p.WEB-DL.AAC2.0.x264-GROUP"),
			},
			want: false,
		},
		{
			name:          "leading zeros",
			episodeTitle:  "ShowName",
			seasonNumber:  1,
			episodeNumber: 2,
			release: &prowlarr.ReleaseResource{
				Title: nullable.NewNullableWithValue("ShowName.S01E02.1080p.WEB-DL.AAC2.0.x264-GROUP"),
			},
			want: false,
		},
		{
			name:          "no season episode info",
			episodeTitle:  "ShowName",
			seasonNumber:  1,
			episodeNumber: 2,
			release: &prowlarr.ReleaseResource{
				Title: nullable.NewNullableWithValue("ShowName.1080p.WEB-DL.AAC2.0.x264-GROUP"),
			},
			want: true,
		},
		{
			name:          "wrong show name",
			episodeTitle:  "ShowName",
			seasonNumber:  1,
			episodeNumber: 2,
			release: &prowlarr.ReleaseResource{
				Title: nullable.NewNullableWithValue("DifferentShow.S01E02.1080p.WEB-DL.AAC2.0.x264-GROUP"),
			},
			want: true,
		},
		{
			name:          "case insensitive match",
			episodeTitle:  "ShowName",
			seasonNumber:  1,
			episodeNumber: 2,
			release: &prowlarr.ReleaseResource{
				Title: nullable.NewNullableWithValue("showname.s01e02.1080p.WEB-DL.AAC2.0.x264-GROUP"),
			},
			want: false,
		},
		{
			name:          "underscore separator",
			episodeTitle:  "ShowName",
			seasonNumber:  1,
			episodeNumber: 2,
			release: &prowlarr.ReleaseResource{
				Title: nullable.NewNullableWithValue("ShowName_S01E02_1080p_WEB-DL_AAC2.0_x264-GROUP"),
			},
			want: false,
		},
		{
			name:          "show name does not match",
			episodeTitle:  "ShowName",
			seasonNumber:  1,
			episodeNumber: 2,
			release: &prowlarr.ReleaseResource{
				Title: nullable.NewNullableWithValue("yo.s01e02.1080p.WEB-DL.AAC2.0.x264-GROUP"),
			},
			want: true,
		},
		{
			name:          "not related release",
			episodeTitle:  "ShowName",
			seasonNumber:  1,
			episodeNumber: 2,
			release: &prowlarr.ReleaseResource{
				Title: nullable.NewNullableWithValue("asdfadsfadsfads"),
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rejectEpisodeReleaseFunc(tt.episodeTitle, tt.seasonNumber, tt.episodeNumber, tt.release)
			assert.Equal(t, tt.want, got)
		})
	}
}

func equalValuesPrettyPrint(t testing.TB, expected, actual any) bool {
	return assert.EqualValues(t, expected, actual, "exp=%v, got=%v", reflect.Indirect(reflect.ValueOf(expected)), reflect.Indirect(reflect.ValueOf(actual)))
}

func assertArrayString(t *testing.T, expected, actual *string) {
	if expected == nil {
		assert.Nil(t, actual)
		return
	}
	expectedWords := strings.Split(*expected, " ")
	actualWords := strings.Split(*actual, " ")
	assert.ElementsMatch(t, expectedWords, actualWords)
}
