package manager

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/kasuboski/mediaz/pkg/prowlarr"
	"github.com/kasuboski/mediaz/pkg/ptr"
	"github.com/kasuboski/mediaz/pkg/storage"
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
		},
		{
			name:     "WEB-DL quality",
			filename: "The.Brothers.Karamazov.1969.(Dostoevsky.Mini.Series).1080p.WEB-DL.x264.Classics",
			want:     "WEBDL-1080p",
		},
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
			want: "Der Untergang Downfall",
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
		{
			name: "TV show with US country code",
			path: "Euphoria (US)",
			want: "Euphoria",
		},
		{
			name: "TV show with UK country code",
			path: "The Office (UK)",
			want: "The Office",
		},
		{
			name: "TV show with AU country code in brackets",
			path: "Offspring [AU]",
			want: "Offspring",
		},
		{
			name: "movie with year and country code",
			path: "Parasite (2019) (KR)",
			want: "Parasite",
		},
		{
			name: "three letter country code",
			path: "Degrassi (CAN)",
			want: "Degrassi",
		},
		{
			name: "country code with year and quality",
			path: "Dark (DE) (2017) 1080p",
			want: "Dark",
		},
		{
			name: "preserve non-country code parentheses",
			path: "Star Trek (The Next Generation)",
			want: "Star Trek (The Next Generation)",
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

func TestNormalizeSeparators(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "dots to spaces",
			input: "Movie.Name.2024",
			want:  "Movie Name 2024",
		},
		{
			name:  "underscores to spaces",
			input: "Movie_Name_2024",
			want:  "Movie Name 2024",
		},
		{
			name:  "dashes to spaces",
			input: "Movie-Name-2024",
			want:  "Movie Name 2024",
		},
		{
			name:  "mixed separators",
			input: "Movie_Name-Here.2024",
			want:  "Movie Name Here 2024",
		},
		{
			name:  "collapse duplicate separators",
			input: "Movie__Name..2024",
			want:  "Movie Name 2024",
		},
		{
			name:  "collapse mixed duplicates",
			input: "Movie._-Name_-.2024",
			want:  "Movie Name 2024",
		},
		{
			name:  "no separators",
			input: "MovieName",
			want:  "MovieName",
		},
		{
			name:  "already spaces",
			input: "Movie Name 2024",
			want:  "Movie Name 2024",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "leading and trailing separators",
			input: ".Movie_Name.",
			want:  " Movie Name ",
		},
		{
			name:  "complex filename",
			input: "The.Dark.Knight.2008.1080p.BluRay",
			want:  "The Dark Knight 2008 1080p BluRay",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeSeparators(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExtractYear(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  *int32
	}{
		{
			name:  "numeric title only is not a year",
			input: "1917",
			want:  nil,
		},
		{
			name:  "numeric title with actual year after",
			input: "2012 2009 1080p",
			want:  ptr.To(int32(2009)),
		},
		{
			name:  "movie title 1917 with release year",
			input: "1917 2019 1080p",
			want:  ptr.To(int32(2019)),
		},
		{
			name:  "year in parentheses",
			input: "Zoolander (2001)",
			want:  ptr.To(int32(2001)),
		},
		{
			name:  "year in square brackets",
			input: "Movie [2020]",
			want:  ptr.To(int32(2020)),
		},
		{
			name:  "year in curly braces",
			input: "Movie {2019}",
			want:  ptr.To(int32(2019)),
		},
		{
			name:  "trailing year after dot separators",
			input: "Movie.Name.2024",
			want:  ptr.To(int32(2024)),
		},
		{
			name:  "trailing year after underscore separators",
			input: "Movie_Name_2023",
			want:  ptr.To(int32(2023)),
		},
		{
			name:  "trailing year after dash separators",
			input: "Movie-Name-2022",
			want:  ptr.To(int32(2022)),
		},
		{
			name:  "no year present",
			input: "Movie Name",
			want:  nil,
		},
		{
			name:  "empty string",
			input: "",
			want:  nil,
		},
		{
			name:  "bracketed year preferred over trailing",
			input: "Movie Name (2019) 2020",
			want:  ptr.To(int32(2019)),
		},
		{
			name:  "year with quality after",
			input: "Movie Name 2017 1080p",
			want:  ptr.To(int32(2017)),
		},
		{
			name:  "dot separated with quality and year",
			input: "Columbus.2017.1080p.WEB-DL.H264",
			want:  ptr.To(int32(2017)),
		},
		{
			name:  "year only in brackets mid string",
			input: "Movie (2019) Extended Edition",
			want:  ptr.To(int32(2019)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractYear(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExtractYearFromPath(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  *int32
	}{
		{
			name:  "numeric title only is not a year",
			input: "1917",
			want:  nil,
		},
		{
			name:  "numeric title with release year",
			input: "2012.2009.1080p",
			want:  ptr.To(int32(2009)),
		},
		{
			name:  "year in parentheses",
			input: "Zoolander (2001)",
			want:  ptr.To(int32(2001)),
		},
		{
			name:  "year in square brackets",
			input: "Guardians of the Galaxy [2014]",
			want:  ptr.To(int32(2014)),
		},
		{
			name:  "year in curly braces",
			input: "Apocalypse Now {1979}",
			want:  ptr.To(int32(1979)),
		},
		{
			name:  "underscore separated",
			input: "Movie_Name_2024_1080p",
			want:  ptr.To(int32(2024)),
		},
		{
			name:  "dash separated",
			input: "Movie-Name-2024",
			want:  ptr.To(int32(2024)),
		},
		{
			name:  "no year",
			input: "Movie Name",
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractYearFromPath(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPathToSearchTermWithYear(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantTerm string
		wantYear *int32
	}{
		{
			name:     "dot separated with year and quality",
			input:    "Columbus.2017.1080p.WEB-DL.H264.AC3-EVO[EtHD]",
			wantTerm: "Columbus",
			wantYear: ptr.To(int32(2017)),
		},
		{
			name:     "underscore separated",
			input:    "Movie_Name_2024_1080p",
			wantTerm: "Movie Name",
			wantYear: ptr.To(int32(2024)),
		},
		{
			name:     "dash separated",
			input:    "Movie-Name-2024",
			wantTerm: "Movie Name",
			wantYear: ptr.To(int32(2024)),
		},
		{
			name:     "parentheses year",
			input:    "Zoolander (2001)",
			wantTerm: "Zoolander",
			wantYear: ptr.To(int32(2001)),
		},
		{
			name:     "numeric title 1917 no year",
			input:    "1917",
			wantTerm: "1917",
			wantYear: nil,
		},
		{
			name:     "numeric title 2012 with actual year",
			input:    "2012.2009.1080p",
			wantTerm: "2012",
			wantYear: ptr.To(int32(2009)),
		},
		{
			name:     "empty string",
			input:    "",
			wantTerm: "",
			wantYear: nil,
		},
		{
			name:     "year and country code",
			input:    "Parasite (2019) (KR)",
			wantTerm: "Parasite",
			wantYear: ptr.To(int32(2019)),
		},
		{
			name:     "alternate title with year",
			input:    "Zoolander (Blue Steel) (2001)",
			wantTerm: "Zoolander (Blue Steel)",
			wantYear: ptr.To(int32(2001)),
		},
		{
			name:     "mixed separators normalized",
			input:    "The.Dark-Knight_2008.1080p",
			wantTerm: "The Dark Knight",
			wantYear: ptr.To(int32(2008)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			term, year := pathToSearchTermWithYear(tt.input)
			assert.Equal(t, tt.wantTerm, term)
			assert.Equal(t, tt.wantYear, year)
		})
	}
}

func TestRejectMovieReleaseFunc(t *testing.T) {
	year2019 := int32(2019)
	params := ReleaseFilterParams{
		Title:   "Movie",
		Year:    &year2019,
		Runtime: 120,
	}
	profile := storage.QualityProfile{}
	protocols := map[string]struct{}{}

	t.Run("nil release", func(t *testing.T) {
		reject := RejectMovieReleaseFunc(context.Background(), params, profile, protocols)
		assert.True(t, reject(nil))
	})

	t.Run("unspecified title", func(t *testing.T) {
		reject := RejectMovieReleaseFunc(context.Background(), params, profile, protocols)
		r := &prowlarr.ReleaseResource{}
		assert.True(t, reject(r))
	})

	t.Run("null title", func(t *testing.T) {
		reject := RejectMovieReleaseFunc(context.Background(), params, profile, protocols)
		r := &prowlarr.ReleaseResource{
			Title: nullable.NewNullNullable[string](),
		}
		assert.True(t, reject(r))
	})

	t.Run("empty title", func(t *testing.T) {
		reject := RejectMovieReleaseFunc(context.Background(), params, profile, protocols)
		r := &prowlarr.ReleaseResource{
			Title: nullable.NewNullableWithValue(""),
		}
		assert.True(t, reject(r))
	})

	t.Run("whitespace only title", func(t *testing.T) {
		reject := RejectMovieReleaseFunc(context.Background(), params, profile, protocols)
		r := &prowlarr.ReleaseResource{
			Title: nullable.NewNullableWithValue("   "),
		}
		assert.True(t, reject(r))
	})
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
