package library

import (
	"context"
	"errors"
	"fmt"
	"maps"
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
	maps.Copy(fs, negatives)

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

func TestMediaLibrary_AddMovie(t *testing.T) {
	t.Run("error making directory", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockfs := mocks.NewMockFileIO(ctrl)

		ctx := context.Background()

		tmpFile, err := os.CreateTemp("testing/", "Batman Begins (2005)-*.mp4")
		if err != nil {
			t.Error(err)
		}

		defer os.Remove(tmpFile.Name())

		movieToAdd := fmt.Sprintf("testing/%s", tmpFile.Name())

		mockfs.EXPECT().MkdirAll(gomock.Any(), gomock.Any()).Times(1).Return(errors.New("expected testing error"))

		fs, _ := MovieFSFromFile(t, "./testing/test_movies.txt")

		fileSystem := FileSystem{
			FS:   fs,
			Path: "testing",
		}

		library := New(FileSystem{}, fileSystem, mockfs)

		title := "Batman Begins"
		movieFile, err := library.AddMovie(ctx, title, movieToAdd)
		wantMovieFile := MovieFile{}
		assert.Error(t, err)
		assert.Equal(t, err.Error(), "expected testing error")
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

		mockfs.EXPECT().MkdirAll(gomock.Any(), gomock.Any()).Times(1).Return(nil)
		mockfs.EXPECT().IsSameFileSystem(gomock.Any(), gomock.Any()).Times(1).Return(false, errors.New("expected testing error"))

		fs, _ := MovieFSFromFile(t, "./testing/test_movies.txt")

		fileSystem := FileSystem{
			FS:   fs,
			Path: "testing",
		}

		library := New(fileSystem, FileSystem{}, mockfs)

		ctx := context.Background()

		title := "Batman Begins"
		movieFile, err := library.AddMovie(ctx, title, movieToAdd)
		wantMovieFile := MovieFile{
			Name:         "",
			Size:         0,
			RelativePath: "",
			AbsolutePath: "",
		}

		assert.Error(t, err)
		assert.Equal(t, err.Error(), "expected testing error")
		assert.Equal(t, wantMovieFile, movieFile)
	})

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

		// Create a mock file info with the Size() method
		mockFileInfo := mocks.NewMockFileInfo(ctrl)
		mockFileInfo.EXPECT().Size().Return(int64(1024)).AnyTimes()

		mockfs.EXPECT().MkdirAll(gomock.Any(), gomock.Any()).Times(1).Return(nil)
		mockfs.EXPECT().IsSameFileSystem(gomock.Any(), gomock.Any()).Times(1).Return(true, nil)
		mockfs.EXPECT().Rename(gomock.Any(), gomock.Any()).Times(1).Return(nil)
		mockfs.EXPECT().Stat(gomock.Any()).Times(1).Return(mockFileInfo, nil)

		fs, _ := MovieFSFromFile(t, "./testing/test_movies.txt")

		fileSystem := FileSystem{
			FS:   fs,
			Path: "testing",
		}

		library := New(fileSystem, FileSystem{}, mockfs)

		title := "Batman Begins"
		movieFile, err := library.AddMovie(ctx, title, movieToAdd)
		wantMovieFile := MovieFile{
			Name:         filepath.Base(tmpFile.Name()),
			Size:         1024,
			RelativePath: filepath.Join(title, filepath.Base(tmpFile.Name())),
			AbsolutePath: filepath.Join(fileSystem.Path, title, filepath.Base(tmpFile.Name())),
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

		// Create a mock file info with the Size() method
		mockFileInfo := mocks.NewMockFileInfo(ctrl)
		mockFileInfo.EXPECT().Size().Return(int64(1024)).AnyTimes()

		mockfs.EXPECT().MkdirAll(gomock.Any(), gomock.Any()).Times(1).Return(nil)
		mockfs.EXPECT().IsSameFileSystem(gomock.Any(), gomock.Any()).Times(1).Return(false, nil)
		mockfs.EXPECT().Copy(gomock.Any(), gomock.Any()).Times(1).Return(int64(0), nil)
		mockfs.EXPECT().Stat(gomock.Any()).Times(1).Return(mockFileInfo, nil)

		fs, _ := MovieFSFromFile(t, "./testing/test_movies.txt")

		fileSystem := FileSystem{
			FS:   fs,
			Path: "testing",
		}

		library := New(fileSystem, FileSystem{}, mockfs)

		ctx := context.Background()
		title := "Batman Begins"
		movieFile, err := library.AddMovie(ctx, title, movieToAdd)
		wantMovieFile := MovieFile{
			Name:         filepath.Base(tmpFile.Name()),
			Size:         1024,
			RelativePath: filepath.Join(title, filepath.Base(tmpFile.Name())),
			AbsolutePath: filepath.Join(fileSystem.Path, title, filepath.Base(tmpFile.Name())),
		}

		assert.Nil(t, err)
		assert.Equal(t, wantMovieFile, movieFile)
	})

	t.Run("same file system - file already exists", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockfs := mocks.NewMockFileIO(ctrl)

		ctx := context.Background()

		tmpFile, err := os.CreateTemp("testing/", "Batman Begins (2005)-*.mp4")
		if err != nil {
			t.Error(err)
		}
		defer os.Remove(tmpFile.Name())

		movieToAdd := fmt.Sprintf("testing/%s", tmpFile.Name())

		// Create a mock file info with the Size() method
		mockFileInfo := mocks.NewMockFileInfo(ctrl)
		mockFileInfo.EXPECT().Size().Return(int64(1024)).AnyTimes()

		mockfs.EXPECT().MkdirAll(gomock.Any(), gomock.Any()).Times(1).Return(nil)
		mockfs.EXPECT().IsSameFileSystem(gomock.Any(), gomock.Any()).Times(1).Return(true, nil)
		mockfs.EXPECT().Rename(gomock.Any(), gomock.Any()).Times(1).Return(io.ErrFileExists)
		mockfs.EXPECT().Stat(gomock.Any()).Times(1).Return(mockFileInfo, nil)

		fs, _ := MovieFSFromFile(t, "./testing/test_movies.txt")

		fileSystem := FileSystem{
			FS:   fs,
			Path: "testing",
		}

		library := New(fileSystem, FileSystem{}, mockfs)

		title := "Batman Begins"
		movieFile, err := library.AddMovie(ctx, title, movieToAdd)
		wantMovieFile := MovieFile{
			Name:         filepath.Base(tmpFile.Name()),
			Size:         1024,
			RelativePath: filepath.Join(title, filepath.Base(tmpFile.Name())),
			AbsolutePath: filepath.Join(fileSystem.Path, title, filepath.Base(tmpFile.Name())),
		}
		assert.Error(t, err)
		assert.True(t, errors.Is(err, io.ErrFileExists))
		assert.Equal(t, wantMovieFile, movieFile)
	})

	t.Run("different file system - file already exists", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockfs := mocks.NewMockFileIO(ctrl)

		tmpFile, err := os.CreateTemp("testing/", "Batman Begins (2005)-*.mp4")
		if err != nil {
			t.Error(err)
		}
		defer os.Remove(tmpFile.Name())

		movieToAdd := fmt.Sprintf("testing/%s", tmpFile.Name())

		// Create a mock file info with the Size() method
		mockFileInfo := mocks.NewMockFileInfo(ctrl)
		mockFileInfo.EXPECT().Size().Return(int64(1024)).AnyTimes()

		mockfs.EXPECT().MkdirAll(gomock.Any(), gomock.Any()).Times(1).Return(nil)
		mockfs.EXPECT().IsSameFileSystem(gomock.Any(), gomock.Any()).Times(1).Return(false, nil)
		mockfs.EXPECT().Copy(gomock.Any(), gomock.Any()).Times(1).Return(int64(0), io.ErrFileExists)
		mockfs.EXPECT().Stat(gomock.Any()).Times(1).Return(mockFileInfo, nil)

		fs, _ := MovieFSFromFile(t, "./testing/test_movies.txt")

		fileSystem := FileSystem{
			FS:   fs,
			Path: "testing",
		}

		library := New(fileSystem, FileSystem{}, mockfs)

		ctx := context.Background()
		title := "Batman Begins"
		movieFile, err := library.AddMovie(ctx, title, movieToAdd)
		wantMovieFile := MovieFile{
			Name:         filepath.Base(tmpFile.Name()),
			Size:         1024,
			RelativePath: filepath.Join(title, filepath.Base(tmpFile.Name())),
			AbsolutePath: filepath.Join(fileSystem.Path, title, filepath.Base(tmpFile.Name())),
		}

		assert.Error(t, err)
		assert.True(t, errors.Is(err, io.ErrFileExists))
		assert.Equal(t, wantMovieFile, movieFile)
	})
}

func TestMediaLibrary_AddEpisode(t *testing.T) {
	t.Run("same file system - success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockfs := mocks.NewMockFileIO(ctrl)

		ctx := context.Background()

		tmpFile, err := os.CreateTemp("testing/", "Series Name - s01e01 - Episode Title.mp4")
		if err != nil {
			t.Error(err)
		}
		defer os.Remove(tmpFile.Name())

		episodeToAdd := fmt.Sprintf("testing/%s", tmpFile.Name())

		// Create a mock file info with the Size() method
		mockFileInfo := mocks.NewMockFileInfo(ctrl)
		mockFileInfo.EXPECT().Size().Return(int64(1024)).AnyTimes()

		mockfs.EXPECT().MkdirAll(gomock.Any(), gomock.Any()).Times(1).Return(nil)
		mockfs.EXPECT().IsSameFileSystem(gomock.Any(), gomock.Any()).Times(1).Return(true, nil)
		mockfs.EXPECT().Rename(gomock.Any(), gomock.Any()).Times(1).Return(nil)
		mockfs.EXPECT().Stat(gomock.Any()).Times(1).Return(mockFileInfo, nil)

		fs, _ := TVFSFromFile(t, "./testing/test_episodes.txt")

		fileSystem := FileSystem{
			FS:   fs,
			Path: "testing",
		}

		library := New(FileSystem{}, fileSystem, mockfs)

		seriesTitle := "Series Name"
		var seasonNumber int32 = 1
		episodeFile, err := library.AddEpisode(ctx, seriesTitle, seasonNumber, episodeToAdd)

		expectedFilename := filepath.Base(tmpFile.Name())
		wantEpisodeFile := EpisodeFile{
			Name:         expectedFilename,
			Size:         1024,
			SeriesTitle:  seriesTitle,
			Season:       seasonNumber,
			RelativePath: fmt.Sprintf("%s/Season %02d/%s", seriesTitle, seasonNumber, expectedFilename),
			AbsolutePath: filepath.Join(fileSystem.Path, seriesTitle, fmt.Sprintf("Season %02d", seasonNumber), expectedFilename),
		}

		assert.Nil(t, err)
		assert.Equal(t, wantEpisodeFile, episodeFile)
	})

	t.Run("same file system - file already exists", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockfs := mocks.NewMockFileIO(ctrl)

		ctx := context.Background()

		tmpFile, err := os.CreateTemp("testing/", "Series Name - s01e01 - Episode Title.mp4")
		if err != nil {
			t.Error(err)
		}
		defer os.Remove(tmpFile.Name())

		episodeToAdd := fmt.Sprintf("testing/%s", tmpFile.Name())

		// Create a mock file info with the Size() method
		mockFileInfo := mocks.NewMockFileInfo(ctrl)
		mockFileInfo.EXPECT().Size().Return(int64(1024)).AnyTimes()

		mockfs.EXPECT().MkdirAll(gomock.Any(), gomock.Any()).Times(1).Return(nil)
		mockfs.EXPECT().IsSameFileSystem(gomock.Any(), gomock.Any()).Times(1).Return(true, nil)
		mockfs.EXPECT().Rename(gomock.Any(), gomock.Any()).Times(1).Return(io.ErrFileExists)
		mockfs.EXPECT().Stat(gomock.Any()).Times(1).Return(mockFileInfo, nil)

		fs, _ := TVFSFromFile(t, "./testing/test_episodes.txt")

		fileSystem := FileSystem{
			FS:   fs,
			Path: "testing",
		}

		library := New(FileSystem{}, fileSystem, mockfs)

		seriesTitle := "Series Name"
		var seasonNumber int32 = 1
		episodeFile, err := library.AddEpisode(ctx, seriesTitle, seasonNumber, episodeToAdd)

		expectedFilename := filepath.Base(tmpFile.Name())
		wantEpisodeFile := EpisodeFile{
			Name:         expectedFilename,
			Size:         1024,
			SeriesTitle:  seriesTitle,
			Season:       seasonNumber,
			RelativePath: fmt.Sprintf("%s/Season %02d/%s", seriesTitle, seasonNumber, expectedFilename),
			AbsolutePath: filepath.Join(fileSystem.Path, seriesTitle, fmt.Sprintf("Season %02d", seasonNumber), expectedFilename),
		}

		assert.Error(t, err)
		assert.True(t, errors.Is(err, io.ErrFileExists))
		assert.Equal(t, wantEpisodeFile, episodeFile)
	})

	t.Run("different file system - file already exists", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		mockfs := mocks.NewMockFileIO(ctrl)

		ctx := context.Background()

		tmpFile, err := os.CreateTemp("testing/", "Series Name - s01e01 - Episode Title.mp4")
		if err != nil {
			t.Error(err)
		}
		defer os.Remove(tmpFile.Name())

		episodeToAdd := fmt.Sprintf("testing/%s", tmpFile.Name())

		// Create a mock file info with the Size() method
		mockFileInfo := mocks.NewMockFileInfo(ctrl)
		mockFileInfo.EXPECT().Size().Return(int64(1024)).AnyTimes()

		mockfs.EXPECT().MkdirAll(gomock.Any(), gomock.Any()).Times(1).Return(nil)
		mockfs.EXPECT().IsSameFileSystem(gomock.Any(), gomock.Any()).Times(1).Return(false, nil)
		mockfs.EXPECT().Copy(gomock.Any(), gomock.Any()).Times(1).Return(int64(0), io.ErrFileExists)
		mockfs.EXPECT().Stat(gomock.Any()).Times(1).Return(mockFileInfo, nil)

		fs, _ := TVFSFromFile(t, "./testing/test_episodes.txt")

		fileSystem := FileSystem{
			FS:   fs,
			Path: "testing",
		}

		library := New(FileSystem{}, fileSystem, mockfs)

		seriesTitle := "Series Name"
		var seasonNumber int32 = 1
		episodeFile, err := library.AddEpisode(ctx, seriesTitle, seasonNumber, episodeToAdd)

		expectedFilename := filepath.Base(tmpFile.Name())
		wantEpisodeFile := EpisodeFile{
			Name:         expectedFilename,
			Size:         1024,
			SeriesTitle:  seriesTitle,
			Season:       seasonNumber,
			RelativePath: fmt.Sprintf("%s/Season %02d/%s", seriesTitle, seasonNumber, expectedFilename),
			AbsolutePath: filepath.Join(fileSystem.Path, seriesTitle, fmt.Sprintf("Season %02d", seasonNumber), expectedFilename),
		}

		assert.Error(t, err)
		assert.True(t, errors.Is(err, io.ErrFileExists))
		assert.Equal(t, wantEpisodeFile, episodeFile)
	})
}
