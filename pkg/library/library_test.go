package library

import (
	"bufio"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"testing/fstest"
)

func TestMatchMovie(t *testing.T) {
	_, expected := fsFromFile(t, "./test_movies.txt")

	for _, m := range expected {
		matched := matchMovie(m)
		if !matched {
			t.Errorf("didn't match movie: %s", m)
		}
	}

	negatives := []string{"Batman Begins (2005).en.srt"}
	for _, m := range negatives {
		matched := matchMovie(m)
		if matched {
			t.Errorf("shouldn't match movie: %s", m)
		}
	}
}

func TestMatchEpisode(t *testing.T) {
	_, expected := fsFromFile(t, "./test_episodes.txt")

	for _, m := range expected {
		matched := matchEpisode(m)
		if !matched {
			t.Errorf("didn't match episode: %s", m)
		}
	}

	negatives := []string{"Batman Begins (2005).en.srt"}
	for _, m := range negatives {
		matched := matchMovie(m)
		if matched {
			t.Errorf("shouldn't match episode: %s", m)
		}
	}
}

func TestFindMovies(t *testing.T) {
	// t.Skip()
	negatives := fstest.MapFS{
		"myfile.txt": {},
		"Batman Begins (2005)/Batman Begins (2005).en.srt": {},
		"My Movie/Uh Oh/My Movie.mp4":                      {},
	}
	fs, expected := fsFromFile(t, "./test_movies.txt")
	for k, v := range negatives {
		fs[k] = v
	}

	l := New(fs, nil)
	movies, err := l.FindMovies()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(expected, movies) {
		t.Fatalf("wanted %v; got %v", expected, movies)
	}

}

func TestFindEpisodes(t *testing.T) {
	// t.Skip()
	fs, expected := fsFromFile(t, "./test_episodes.txt")
	fs["myfile.txt"] = &fstest.MapFile{}

	l := New(nil, fs)
	episodes := l.FindEpisodes()
	if !slices.Equal(expected, episodes) {
		t.Fatalf("wanted %v; got %v", expected, episodes)
	}
}

func fsFromFile(t *testing.T, path string) (fstest.MapFS, []string) {
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
		names = append(names, filepath.Base(path))
	}

	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}

	return testfs, names
}
