package manager

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

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
			// don't worry about title and year for now; maybe we similarity search them later
			// assert.Equal(t, tc.Title, parsed.Title)
			// equalValuesPrettyPrint(t, tc.Year, parsed.Year)
			equalValuesPrettyPrint(t, tc.Edition, parsed.Edition)
			equalValuesPrettyPrint(t, tc.Customformat, parsed.Customformat)
			equalValuesPrettyPrint(t, tc.Quality, parsed.Quality)
			equalValuesPrettyPrint(t, tc.Mediainfo3D, parsed.Mediainfo3D)
			equalValuesPrettyPrint(t, tc.MediainfoDynamicrange, parsed.MediainfoDynamicrange)
			equalValuesPrettyPrint(t, tc.MediainfoAudio, parsed.MediainfoAudio)
			equalValuesPrettyPrint(t, tc.MediainfoVideo, parsed.MediainfoVideo)
			equalValuesPrettyPrint(t, tc.Releasegroup, parsed.Releasegroup)
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

func equalValuesPrettyPrint(t testing.TB, expected, actual interface{}) bool {
	return assert.EqualValues(t, expected, actual, "exp=%v, got=%v", reflect.Indirect(reflect.ValueOf(expected)), reflect.Indirect(reflect.ValueOf(actual)))
}
