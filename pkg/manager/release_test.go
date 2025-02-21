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
			assert.Equal(t, tc.Title, parsed.Title)
			equalValuesPrettyPrint(t, tc.Year, parsed.Year)
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
