package library

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEpisodeFileFromPath(t *testing.T) {
	f, err := os.Open("./testing/test_episodes.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		p := scanner.Text()
		ef := EpisodeFileFromPath(p)
		base := filepath.Base(p)
		if ef.Name != base {
			t.Fatalf("Name mismatch for %s: want %s got %s", p, base, ef.Name)
		}
		series := sanitizeName(dirName(filepath.Dir(p)))
		if ef.SeriesName != series {
			t.Fatalf("SeriesName mismatch for %s: want %s got %s", p, series, ef.SeriesName)
		}
		season := 0
		parent := dirName(p)
		if strings.HasPrefix(strings.ToLower(parent), "season ") {
			var n int
			fmt.Sscanf(parent, "Season %d", &n)
			season = n
		}
		if ef.SeasonNumber != season {
			t.Fatalf("SeasonNumber mismatch for %s: want %d got %d", p, season, ef.SeasonNumber)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
}
