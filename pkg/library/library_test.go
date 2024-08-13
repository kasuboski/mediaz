package library

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"testing/fstest"
)

func TestMatchMovie(t *testing.T) {
	_, expected := fsFromFile(t, "./test_movies.txt", dirName)

	for _, m := range expected {
		matched := matchMovie(m)
		if !matched {
			t.Errorf("didn't match movie: %s", m)
		}
	}
}

func TestMatchEpisode(t *testing.T) {
	_, expected := fsFromFile(t, "./test_episodes.txt", filepath.Base)

	for _, m := range expected {
		matched := matchEpisode(m)
		if !matched {
			t.Errorf("didn't match episode: %s", m)
		}
	}
}

func TestFindMovies(t *testing.T) {
	ctx := context.Background()
	negatives := fstest.MapFS{
		"myfile.txt": {},
		"Batman Begins (2005)/Batman Begins (2005).en.srt": {},
		"My Movie/Uh Oh/My Movie.mp4":                      {},
	}
	fs, expected := fsFromFile(t, "./test_movies.txt", dirName)
	for k, v := range negatives {
		fs[k] = v
	}

	l := New(fs, nil)
	movies, err := l.FindMovies(ctx)
	if err != nil {
		t.Fatal(err)
	}
	movieNames := make([]string, len(movies))
	for i, m := range movies {
		movieNames[i] = m.Name
	}
	if !slices.Equal(expected, movieNames) {
		t.Fatalf("wanted %v; got %v", expected, movieNames)
	}

}

func TestFindEpisodes(t *testing.T) {
	ctx := context.Background()
	fs, expected := fsFromFile(t, "./test_episodes.txt", filepath.Base)
	fs["myfile.txt"] = &fstest.MapFile{}

	l := New(nil, fs)
	episodes, err := l.FindEpisodes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(expected, episodes) {
		t.Fatalf("wanted %v; got %v", expected, episodes)
	}
}

func fsFromFile(t *testing.T, path string, toExpected func(string) string) (fstest.MapFS, []string) {
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("couldn't open file: %v", err)
	}
	defer f.Close()

	testfs := fstest.MapFS{}
	names := []string{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		path := scanner.Text()
		testfs[path] = &fstest.MapFile{}
		names = append(names, toExpected(path))
	}

	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}

	return testfs, names
}
