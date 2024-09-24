package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/stretchr/testify/assert"
)

func TestInit(t *testing.T) {
	store := initSqlite(t, context.Background())
	assert.NotNil(t, store)
}

func TestIndexerStorage(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)
	assert.NotNil(t, store)

	ix, err := store.ListIndexers(ctx)
	assert.Nil(t, err)
	assert.Empty(t, ix)

	apikey := "supersecret"
	create := model.Indexer{
		ID:       1,
		Name:     "Index",
		Priority: 20,
		URI:      "http://here",
		APIKey:   &apikey,
	}
	res, err := store.CreateIndexer(ctx, create)
	assert.Nil(t, err)
	assert.NotEmpty(t, res)

	ix, err = store.ListIndexers(ctx)
	assert.Nil(t, err)
	assert.Len(t, ix, 1)
	actual := ix[0]
	assert.Equal(t, &create, actual)

	err = store.DeleteIndexer(ctx, int64(actual.ID))
	assert.Nil(t, err)

	ix, err = store.ListIndexers(ctx)
	assert.Nil(t, err)
	assert.Empty(t, ix)
}

func TestMovieStorage(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)
	assert.NotNil(t, store)

	movie := model.Movies{
		ID:              1,
		Path:            "Title/Title.mkv",
		Monitored:       1,
		MovieFileId:     1,
		MovieMetadataId: 1,
	}
	res, err := store.CreateMovie(ctx, movie)
	assert.Nil(t, err)
	assert.NotEmpty(t, res)

	id := int64(res)
	movies, err := store.ListMovies(ctx)
	assert.Nil(t, err)
	assert.Len(t, movies, 1)
	actual := movies[0]
	assert.Equal(t, &movie, actual)

	err = store.DeleteMovie(ctx, id)
	assert.Nil(t, err)

	movies, err = store.ListMovies(ctx)
	assert.Nil(t, err)
	assert.Empty(t, movies)

	file := model.MovieFiles{
		ID:      1,
		Quality: "HDTV-720p",
		Size:    1_000_000_000,
		MovieId: 1,
	}
	res, err = store.CreateMovieFile(ctx, file)
	assert.Nil(t, err)
	assert.NotEmpty(t, res)

	id = int64(res)
	files, err := store.ListMovieFiles(ctx)
	assert.Nil(t, err)
	assert.Len(t, files, 1)
	actualFile := files[0]
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

func initSqlite(t *testing.T, ctx context.Context) storage.Storage {
	store, err := New(":memory:")
	assert.Nil(t, err)

	schemas, err := storage.ReadSchemaFiles("./schema/schema.sql")
	assert.Nil(t, err)

	err = store.Init(ctx, schemas...)
	assert.Nil(t, err)
	return store
}
