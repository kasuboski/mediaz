package manager

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestQualitySizeExample(t *testing.T) {
	readMovieFile(t)
}

func TestQualitySizeCutoff(t *testing.T) {
	qs := QualitySize{
		Quality:   "HDTV-720p",
		Min:       17.1,
		Preferred: 1999,
		Max:       2000,
	}

	tests := []struct {
		size    int
		runtime int
		want    bool
	}{
		{
			1000,
			60,
			false,
		},
		{
			1026,
			60,
			true,
		},
		{
			120_000,
			60,
			true,
		},
		{
			120_001,
			60,
			false,
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d,%d", tt.size, tt.runtime), func(t *testing.T) {
			if got := MeetsQualitySize(qs, tt.size, tt.runtime); got != tt.want {
				t.Errorf("got %v; want %v", got, tt.want)
			}
		})
	}
}

func readMovieFile(t *testing.T) QualitySizes {
	// https://github.com/TRaSH-Guides/Guides/blob/b7e72827ad96aa3158f479523c07e257ab6cbb09/docs/json/radarr/quality-size/movie.json
	b, err := os.ReadFile("./movieQualitySize.json")
	if err != nil {
		t.Error(err)
	}

	var qs QualitySizes
	err = json.Unmarshal(b, &qs)
	if err != nil {
		t.Error(err)
	}
	return qs
}
