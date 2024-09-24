package library

import (
	"bufio"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func MovieFSFromFile(t *testing.T, path string) (fstest.MapFS, []string) {
	return fsFromFile(t, path, dirName)
}

func TVFSFromFile(t *testing.T, path string) (fstest.MapFS, []string) {
	return fsFromFile(t, path, filepath.Base)
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
