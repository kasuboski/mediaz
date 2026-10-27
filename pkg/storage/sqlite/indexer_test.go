package sqlite

import (
	"context"
	"database/sql"
	"testing"

	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
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
	create := sqlcdb.Indexer{
		ID:         1,
		Name:       "Index",
		Priority:   20,
		Uri:        "http://here",
		ApiKey:     sql.NullString{String: apikey, Valid: true},
		Categories: "[]",
	}
	res, err := store.CreateIndexer(ctx, create)
	assert.Nil(t, err)
	assert.NotEmpty(t, res)

	ix, err = store.ListIndexers(ctx)
	assert.Nil(t, err)
	assert.Len(t, ix, 1)
	actual := ix[0]
	assert.Equal(t, create, actual)

	err = store.DeleteIndexer(ctx, int64(actual.ID))
	assert.Nil(t, err)

	ix, err = store.ListIndexers(ctx)
	assert.Nil(t, err)
	assert.Empty(t, ix)
}

func TestListIndexersOrderByPriorityDesc(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)

	for _, priority := range []int64{10, 99, 50} {
		_, err := store.CreateIndexer(ctx, sqlcdb.Indexer{
			Name:     "indexer",
			Priority: priority,
			Uri:      "http://here",
		})
		require.NoError(t, err)
	}

	indexers, err := store.ListIndexers(ctx)
	require.NoError(t, err)
	require.Len(t, indexers, 3)
	assert.Equal(t, int64(99), indexers[0].Priority)
	assert.Equal(t, int64(50), indexers[1].Priority)
	assert.Equal(t, int64(10), indexers[2].Priority)
}

func TestCreateIndexerConflictRefreshesFields(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)

	sourceID, err := store.CreateIndexerSource(ctx, sqlcdb.IndexerSource{
		Name: "prowlarr", Implementation: "prowlarr", Scheme: "http", Host: "localhost", Enabled: true,
	})
	require.NoError(t, err)
	create := sqlcdb.Indexer{
		Name:            "prowlarr",
		Priority:        25,
		Uri:             "http://prowlarr",
		IndexerSourceID: sql.NullInt64{Int64: sourceID, Valid: true},
		Categories:      "[2000]",
	}

	id, err := store.CreateIndexer(ctx, create)
	require.NoError(t, err)
	assert.NotZero(t, id)

	// The same (source, name) pair refreshes configuration without changing ID.
	create.Priority = 99
	create.Uri = "https://prowlarr.updated"
	create.ApiKey = sql.NullString{String: "refreshed-key", Valid: true}
	create.Categories = "[2040]"
	existingID, err := store.CreateIndexer(ctx, create)
	require.NoError(t, err)
	assert.Equal(t, id, existingID)

	indexers, err := store.ListIndexers(ctx)
	require.NoError(t, err)
	expected := create
	expected.ID = id
	assert.Equal(t, []sqlcdb.Indexer{expected}, indexers)

	// Indexers without a source have no uniqueness constraint on name alone
	// (NULL source ids are distinct), so both rows are stored.
	create.IndexerSourceID = sql.NullInt64{}
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
	id, err := store.CreateIndexer(ctx, sqlcdb.Indexer{
		Name:       "indexer",
		Priority:   25,
		Uri:        "http://here",
		ApiKey:     sql.NullString{String: apiKey, Valid: true},
		Categories: "[2040]",
	})
	require.NoError(t, err)

	// Updating without an API key must preserve the stored key.
	err = store.UpdateIndexer(ctx, id, sqlcdb.Indexer{
		Name:     "renamed",
		Priority: 10,
		Uri:      "http://there",
	})
	require.NoError(t, err)

	indexers, err := store.ListIndexers(ctx)
	require.NoError(t, err)
	require.Len(t, indexers, 1)
	assert.Equal(t, "renamed", indexers[0].Name)
	assert.Equal(t, int64(10), indexers[0].Priority)
	assert.Equal(t, "http://there", indexers[0].Uri)
	require.True(t, indexers[0].ApiKey.Valid)
	assert.Equal(t, "original-key", indexers[0].ApiKey.String)
	assert.Equal(t, "[2040]", indexers[0].Categories, "omitting categories preserves stored categories")

	// Updating with an API key replaces it.
	newKey := "new-key"
	err = store.UpdateIndexer(ctx, id, sqlcdb.Indexer{
		Name:     "renamed",
		Priority: 10,
		Uri:      "http://there",
		ApiKey:   sql.NullString{String: newKey, Valid: true},
	})
	require.NoError(t, err)

	indexers, err = store.ListIndexers(ctx)
	require.NoError(t, err)
	require.Len(t, indexers, 1)
	require.True(t, indexers[0].ApiKey.Valid)
	assert.Equal(t, "new-key", indexers[0].ApiKey.String)
}
