package sqlite

import (
	"context"
	"testing"

	"github.com/kasuboski/mediaz/pkg/storage/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestListIndexersOrderByPriorityDesc(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)

	for _, priority := range []int32{10, 99, 50} {
		_, err := store.CreateIndexer(ctx, model.Indexer{
			Name:     "indexer",
			Priority: priority,
			URI:      "http://here",
		})
		require.NoError(t, err)
	}

	indexers, err := store.ListIndexers(ctx)
	require.NoError(t, err)
	require.Len(t, indexers, 3)
	assert.Equal(t, int32(99), indexers[0].Priority)
	assert.Equal(t, int32(50), indexers[1].Priority)
	assert.Equal(t, int32(10), indexers[2].Priority)
}

func TestCreateIndexerConflictDoesNothing(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)

	sourceID := int32(7)
	create := model.Indexer{
		Name:            "prowlarr",
		Priority:        25,
		URI:             "http://prowlarr",
		IndexerSourceID: &sourceID,
	}

	id, err := store.CreateIndexer(ctx, create)
	require.NoError(t, err)
	assert.NotZero(t, id)

	// Same (source, name) pair: the insert does nothing and does not error,
	// matching the previous on-conflict-do-nothing behavior.
	id, err = store.CreateIndexer(ctx, create)
	require.NoError(t, err)

	indexers, err := store.ListIndexers(ctx)
	require.NoError(t, err)
	assert.Len(t, indexers, 1)

	// Indexers without a source have no uniqueness constraint on name alone
	// (NULL source ids are distinct), so both rows are stored.
	create.IndexerSourceID = nil
	_, err = store.CreateIndexer(ctx, create)
	require.NoError(t, err)
	_, err = store.CreateIndexer(ctx, create)
	require.NoError(t, err)

	indexers, err = store.ListIndexers(ctx)
	require.NoError(t, err)
	assert.Len(t, indexers, 3)
}

func TestUpdateIndexer(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)

	apiKey := "original-key"
	id, err := store.CreateIndexer(ctx, model.Indexer{
		Name:     "indexer",
		Priority: 25,
		URI:      "http://here",
		APIKey:   &apiKey,
	})
	require.NoError(t, err)

	// Updating without an API key must preserve the stored key.
	err = store.UpdateIndexer(ctx, id, model.Indexer{
		Name:     "renamed",
		Priority: 10,
		URI:      "http://there",
	})
	require.NoError(t, err)

	indexers, err := store.ListIndexers(ctx)
	require.NoError(t, err)
	require.Len(t, indexers, 1)
	assert.Equal(t, "renamed", indexers[0].Name)
	assert.Equal(t, int32(10), indexers[0].Priority)
	assert.Equal(t, "http://there", indexers[0].URI)
	require.NotNil(t, indexers[0].APIKey)
	assert.Equal(t, "original-key", *indexers[0].APIKey)

	// Updating with an API key replaces it.
	newKey := "new-key"
	err = store.UpdateIndexer(ctx, id, model.Indexer{
		Name:     "renamed",
		Priority: 10,
		URI:      "http://there",
		APIKey:   &newKey,
	})
	require.NoError(t, err)

	indexers, err = store.ListIndexers(ctx)
	require.NoError(t, err)
	require.Len(t, indexers, 1)
	require.NotNil(t, indexers[0].APIKey)
	assert.Equal(t, "new-key", *indexers[0].APIKey)
}
