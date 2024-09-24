package sqlite

import (
	"context"
	"testing"

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

func initSqlite(t *testing.T, ctx context.Context) storage.Storage {
	store, err := New(":memory:")
	assert.Nil(t, err)

	schemas, err := storage.ReadSchemaFiles("./schema/schema.sql")
	assert.Nil(t, err)

	err = store.Init(ctx, schemas...)
	assert.Nil(t, err)
	return store
}
