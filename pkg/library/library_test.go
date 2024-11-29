package library

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/kasuboski/mediaz/pkg/io"
	"github.com/kasuboski/mediaz/pkg/io/mocks"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestMatchMovie(t *testing.T) {
	_, expected := MovieFSFromFile(t, "./testing/test_movies.txt")

	for _, m := range expected {
		matched := matchMovie(m)
		if !matched {
			t.Errorf("didn't match movie: %s", m)
		}
	}
}

func TestMatchEpisode(t *testing.T) {
	_, expected := TVFSFromFile(t, "./testing/test_episodes.txt")

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
	fs, expected := MovieFSFromFile(t, "./testing/test_movies.txt")
	for k, v := range negatives {
		fs[k] = v
	}

	fileSystem := FileSystem{
		FS: fs,
	}

	l := New(fileSystem, FileSystem{}, &io.MediaFileSystem{})
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
	fs, expected := TVFSFromFile(t, "./testing/test_episodes.txt")
	fs["myfile.txt"] = &fstest.MapFile{}

	fileSystem := FileSystem{
		FS: fs,
	}

	l := New(FileSystem{}, fileSystem, &io.MediaFileSystem{})
	episodes, err := l.FindEpisodes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(expected, episodes) {
		t.Fatalf("wanted %v; got %v", expected, episodes)
	}
}

func Test_fileSizeToString(t *testing.T) {
	type args struct {
		size int64
	}
	tests := []struct {
		name string
		want string
		args args
	}{
		{
			name: "b",
			args: args{size: 500},
			want: "500 B",
		},
		{
			name: "kb",
			args: args{size: 1024},
			want: "1.0 KB",
		},
		{
			name: "mb",
			args: args{size: 1048576}, // 1024 * 1024
			want: "1.0 MB",
		},
		{
			name: "gb",
			args: args{size: 1073741824}, // 1024 * 1024 * 1024
			want: "1.0 GB",
		},
		{
			name: "tb",
			args: args{size: 1099511627776}, // 1024^4
			want: "1.0 TB",
		},
		{
			name: "not whole number",
			args: args{size: 123456789},
			want: "117.7 MB",
		},
		{
			name: "zero",
			args: args{size: 0},
			want: "0 B",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fileSizeToString(tt.args.size); got != tt.want {
				t.Errorf("fileSizeToString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMediaLibrary_AddMovie(t *testing.T) {
	t.Run("same file system", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockfs := mocks.NewMockFileIO(ctrl)

		ctx := context.Background()

		tmpFile, err := os.CreateTemp("testing/", "Batman Begins (2005)-*.mp4")
		if err != nil {
			t.Error(err)
		}

		defer os.Remove(tmpFile.Name())

		movieToAdd := fmt.Sprintf("testing/%s", tmpFile.Name())

		mockfs.EXPECT().IsSameFileSystem(gomock.Any(), gomock.Any()).Times(1).Return(true, nil)
		mockfs.EXPECT().Rename(gomock.Any(), gomock.Any()).Times(1).Return(nil)
		mockfs.EXPECT().Open(gomock.Any()).Times(1).Return(tmpFile, nil)

		fs, _ := MovieFSFromFile(t, "./testing/test_movies.txt")

		fileSystem := FileSystem{
			FS:   fs,
			Path: "testing",
		}

		library := New(FileSystem{}, fileSystem, mockfs)

		movieFile, err := library.AddMovie(ctx, movieToAdd)
		wantMovieFile := MovieFile{
			Name: filepath.Base(tmpFile.Name()),
			Size: 0,
			Path: tmpFile.Name(),
		}
		assert.Nil(t, err)
		assert.Equal(t, wantMovieFile, movieFile)
	})

	t.Run("different file system - success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockfs := mocks.NewMockFileIO(ctrl)

		tmpFile, err := os.CreateTemp("testing/", "Batman Begins (2005)-*.mp4")
		if err != nil {
			t.Error(err)
		}
		defer os.Remove(tmpFile.Name())

		movieToAdd := fmt.Sprintf("testing/%s", tmpFile.Name())

		mockfs.EXPECT().IsSameFileSystem(gomock.Any(), gomock.Any()).Times(1).Return(false, nil)

		var response int64 = 0
		mockfs.EXPECT().Copy(gomock.Any(), gomock.Any()).Times(1).Return(response, nil)
		mockfs.EXPECT().Open(gomock.Any()).Times(1).Return(tmpFile, nil)

		fs, _ := MovieFSFromFile(t, "./testing/test_movies.txt")

		fileSystem := FileSystem{
			FS:   fs,
			Path: "testing",
		}

		library := New(FileSystem{}, fileSystem, mockfs)

		ctx := context.Background()
		movieFile, err := library.AddMovie(ctx, movieToAdd)
		wantMovieFile := MovieFile{
			Name: filepath.Base(tmpFile.Name()),
			Size: 0,
			Path: tmpFile.Name(),
		}

		assert.Nil(t, err)
		assert.Equal(t, wantMovieFile, movieFile)
	})

	t.Run("different file system - error checking if same file system", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockfs := mocks.NewMockFileIO(ctrl)

		tmpFile, err := os.CreateTemp("testing/", "Batman Begins (2005)-*.mp4")
		if err != nil {
			t.Error(err)
		}
		defer os.Remove(tmpFile.Name())

		movieToAdd := fmt.Sprintf("testing/%s", tmpFile.Name())

		mockfs.EXPECT().IsSameFileSystem(gomock.Any(), gomock.Any()).Times(1).Return(false, errors.New("expected testing error"))

		var response int64 = 0
		mockfs.EXPECT().Copy(gomock.Any(), gomock.Any()).Times(1).Return(response, nil)
		mockfs.EXPECT().Open(gomock.Any()).Times(1).Return(tmpFile, nil)

		fs, _ := MovieFSFromFile(t, "./testing/test_movies.txt")

		fileSystem := FileSystem{
			FS:   fs,
			Path: "testing",
		}

		library := New(FileSystem{}, fileSystem, mockfs)

		ctx := context.Background()
		movieFile, err := library.AddMovie(ctx, movieToAdd)
		wantMovieFile := MovieFile{
			Name: filepath.Base(tmpFile.Name()),
			Size: 0,
			Path: tmpFile.Name(),
		}

		assert.Nil(t, err)
		assert.Equal(t, wantMovieFile, movieFile)
	})
}
