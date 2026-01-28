package sqlite

import (
	"context"
	"testing"

	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/stretchr/testify/assert"
)

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
