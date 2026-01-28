package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMovieStorage(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)
	assert.NotNil(t, store)

	path := "Title"
	movie := storage.Movie{
		Movie: model.Movie{
			ID:              1,
			Path:            &path,
			Monitored:       1,
			MovieFileID:     ptr(int32(1)),
			MovieMetadataID: ptr(int32(1)),
		},
	}

	wantMovie := storage.Movie{
		Movie: model.Movie{
			ID:              1,
			Path:            &path,
			Monitored:       1,
			MovieFileID:     ptr(int32(1)),
			MovieMetadataID: ptr(int32(1)),
		},
		State: storage.MovieStateMissing,
	}

	res, err := store.CreateMovie(ctx, movie, storage.MovieStateMissing)
	assert.Nil(t, err)
	assert.NotEmpty(t, res)

	id := int64(res)
	movies, err := store.ListMovies(ctx)
	assert.Nil(t, err)
	assert.Len(t, movies, 1)
	actual := movies[0]
	actual.Added = nil
	assert.Equal(t, &wantMovie, actual)

	movies, err = store.ListMovies(ctx)
	assert.Nil(t, err)
	assert.Len(t, movies, 1)
	actual = movies[0]
	actual.Added = nil
	assert.Equal(t, &wantMovie, actual)

	err = store.UpdateMovieState(ctx, int64(movies[0].ID), storage.MovieStateDownloading, &storage.TransitionStateMetadata{
		DownloadID:       ptr("123"),
		DownloadClientID: ptr(int32(1)),
	})
	assert.Nil(t, err)

	movies, err = store.ListMovies(ctx)
	assert.Nil(t, err)
	assert.Len(t, movies, 1)
	actual = movies[0]
	wantMovie.State = storage.MovieStateDownloading
	wantMovie.DownloadClientID = 1
	wantMovie.DownloadID = "123"
	actual.Added = nil
	assert.Equal(t, &wantMovie, actual)

	movies, err = store.ListMoviesByState(ctx, storage.MovieStateDownloading)
	assert.Nil(t, err)
	assert.Len(t, movies, 1)
	movies[0].Added = nil
	assert.Equal(t, &wantMovie, movies[0])

	mov, err := store.GetMovieByMetadataID(ctx, 1)
	assert.NoError(t, err)
	assert.NotNil(t, mov)

	err = store.DeleteMovie(ctx, id)
	assert.Nil(t, err)

	movies, err = store.ListMovies(ctx)
	assert.Empty(t, movies)
	assert.Nil(t, err)

	file := model.MovieFile{
		ID:           1,
		Quality:      "HDTV-720p",
		Size:         1_000_000_000,
		RelativePath: ptr("Title/Title.mkv"),
	}
	res, err = store.CreateMovieFile(ctx, file)
	assert.Nil(t, err)
	assert.NotEmpty(t, res)

	movie.ID = 2
	mRes, err := store.CreateMovie(ctx, movie, storage.MovieStateMissing)
	assert.Nil(t, err)
	assert.NotEmpty(t, mRes)

	files, err := store.GetMovieFilesByMovieName(ctx, "Title")
	require.NoError(t, err)
	actualFile := files[0]
	actualFile.DateAdded = time.Time{}
	// clear non-deterministic date field
	assert.Equal(t, &file, actualFile)

	id = int64(res)
	files, err = store.ListMovieFiles(ctx)
	assert.Nil(t, err)
	assert.Len(t, files, 1)
	actualFile = files[0]
	assert.NotEmpty(t, actualFile.DateAdded)
	// clear non-deterministic date field
	actualFile.DateAdded = time.Time{}
	assert.Equal(t, &file, actualFile)

	err = store.DeleteMovieFile(ctx, id)
	assert.Nil(t, err)

	files, err = store.ListMovieFiles(ctx)
	assert.Nil(t, err)
	assert.Empty(t, files)
}

func TestSQLite_UpdateMovieMovieFileID(t *testing.T) {
	t.Run("update movie file id", func(t *testing.T) {
		ctx := context.Background()
		store := initSqlite(t, ctx)
		require.NotNil(t, store)

		path := "Title/Title.mkv"
		newMovie := storage.Movie{
			Movie: model.Movie{
				ID:          1,
				Monitored:   1,
				Path:        &path,
				MovieFileID: ptr(int32(1)),
			},
		}

		movieID, err := store.CreateMovie(ctx, newMovie, storage.MovieStateDiscovered)
		require.NoError(t, err)
		require.Equal(t, int64(1), movieID)

		err = store.UpdateMovieMovieFileID(ctx, movieID, 2)
		require.NoError(t, err)

		movie, err := store.GetMovie(ctx, movieID)
		require.NoError(t, err)

		assert.Equal(t, int32(1), movie.ID)
		assert.Equal(t, int32(2), *movie.MovieFileID)
	})
}

func TestSQLite_UpdateMovieQualityProfile(t *testing.T) {
	t.Run("updates only quality profile id", func(t *testing.T) {
		ctx := context.Background()
		store := initSqlite(t, ctx)
		require.NotNil(t, store)

		path := "Title/Title.mkv"
		newMovie := storage.Movie{
			Movie: model.Movie{
				Monitored:        1,
				QualityProfileID: 1,
				Path:             &path,
			},
		}

		movieID, err := store.CreateMovie(ctx, newMovie, storage.MovieStateDiscovered)
		require.NoError(t, err)

		err = store.UpdateMovieQualityProfile(ctx, movieID, 3)
		require.NoError(t, err)

		movie, err := store.GetMovie(ctx, movieID)
		require.NoError(t, err)

		assert.Equal(t, int32(1), movie.Monitored, "Monitored should remain unchanged")
		assert.Equal(t, int32(3), movie.QualityProfileID, "QualityProfileID should be updated")
	})

	t.Run("no error for non-existent movie", func(t *testing.T) {
		ctx := context.Background()
		store := initSqlite(t, ctx)
		require.NotNil(t, store)

		err := store.UpdateMovieQualityProfile(ctx, 999, 3)
		assert.NoError(t, err, "UpdateMovieQualityProfile should not error for non-existent movie")
	})
}

func TestSQLite_GetMovieByMovieFileID(t *testing.T) {
	t.Run("get movie by movie file id", func(t *testing.T) {
		ctx := context.Background()
		store := initSqlite(t, ctx)
		require.NotNil(t, store)

		path := "Title/Title.mkv"
		movie1 := storage.Movie{
			Movie: model.Movie{
				Monitored:   1,
				Path:        &path,
				MovieFileID: ptr(int32(1)),
			},
		}

		_, err := store.CreateMovie(ctx, movie1, storage.MovieStateDiscovered)
		require.NoError(t, err)

		movie2 := storage.Movie{
			Movie: model.Movie{
				Monitored:   1,
				Path:        &path,
				MovieFileID: ptr(int32(2)),
			},
		}
		_, err = store.CreateMovie(ctx, movie2, storage.MovieStateDiscovered)
		require.NoError(t, err)

		movie, err := store.GetMovieByMovieFileID(ctx, 1)
		require.NoError(t, err)

		assert.Equal(t, int32(1), movie.ID)
		assert.Equal(t, int32(1), int32(*movie.MovieFileID))
	})
}
