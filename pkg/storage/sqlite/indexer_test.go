package sqlite

import (
	"context"
	"database/sql"
	"testing"

	"github.com/kasuboski/mediaz/pkg/storage"
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
		Categories: `{"movie":[],"tv":[]}`,
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
		ProwlarrID:      sql.NullInt64{Int64: 10, Valid: true},
		Priority:        25,
		Uri:             "http://prowlarr",
		IndexerSourceID: sql.NullInt64{Int64: sourceID, Valid: true},
		Categories:      `{"movie":[2000],"tv":[]}`,
	}

	id, err := store.CreateIndexer(ctx, create)
	require.NoError(t, err)
	assert.NotZero(t, id)

	// The same (source, Prowlarr ID) pair refreshes configuration without changing ID.
	create.Name = "renamed prowlarr"
	create.Priority = 99
	create.Uri = "https://prowlarr.updated"
	create.ApiKey = sql.NullString{String: "refreshed-key", Valid: true}
	create.Categories = `{"movie":[2040],"tv":[]}`
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

func TestSourceBackedIndexerUpsertUsesProwlarrIDAndAllowsRename(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)
	sourceID, err := store.CreateIndexerSource(ctx, sqlcdb.IndexerSource{Name: "source", Implementation: "prowlarr", Scheme: "http", Host: "localhost", Enabled: true})
	require.NoError(t, err)
	mk := func(name string, externalID int64) sqlcdb.Indexer {
		return sqlcdb.Indexer{Name: name, Uri: "http://" + name, IndexerSourceID: sql.NullInt64{Int64: sourceID, Valid: true}, ProwlarrID: sql.NullInt64{Int64: externalID, Valid: true}}
	}
	id1, err := store.CreateIndexer(ctx, mk("same-name", 101))
	require.NoError(t, err)
	id2, err := store.CreateIndexer(ctx, mk("same-name", 202))
	require.NoError(t, err)
	require.NotEqual(t, id1, id2)
	upsertedID, err := store.CreateIndexer(ctx, mk("renamed", 101))
	require.NoError(t, err)
	assert.Equal(t, id1, upsertedID)
	rows, err := store.ListIndexers(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, "renamed", rows[0].Name)
	assert.Equal(t, int64(101), rows[0].ProwlarrID.Int64)
}

func TestCreateIndexerDefaultsCategoriesCanonical(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)
	id, err := store.CreateIndexer(ctx, sqlcdb.Indexer{Name: "default", Uri: "http://here"})
	require.NoError(t, err)
	result, err := store.(*SQLite).db.ExecContext(ctx, `INSERT INTO indexer (name, uri) VALUES ('schema-default', 'http://schema')`)
	require.NoError(t, err)
	schemaID, err := result.LastInsertId()
	require.NoError(t, err)
	rows, err := store.ListIndexers(ctx)
	require.NoError(t, err)
	assert.Equal(t, []sqlcdb.Indexer{
		{ID: schemaID, Name: "schema-default", Uri: "http://schema", Priority: 25, Categories: `{"movie":[],"tv":[]}`},
		{ID: id, Name: "default", Uri: "http://here", Categories: `{"movie":[],"tv":[]}`},
	}, rows)
}

func TestUpdateIndexer(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)

	sourceID, err := store.CreateIndexerSource(ctx, sqlcdb.IndexerSource{
		Name: "source", Implementation: "prowlarr", Scheme: "http", Host: "localhost", Enabled: true,
	})
	require.NoError(t, err)
	apiKey := "original-key"
	id, err := store.CreateIndexer(ctx, sqlcdb.Indexer{
		Name:            "indexer",
		Priority:        25,
		ProwlarrID:      sql.NullInt64{Int64: 1, Valid: true},
		Uri:             "http://here",
		ApiKey:          sql.NullString{String: apiKey, Valid: true},
		IndexerSourceID: sql.NullInt64{Int64: sourceID, Valid: true},
		Categories:      `{"movie":[2040],"tv":[]}`,
	})
	require.NoError(t, err)

	// Source-backed records are managed by their source and cannot be edited here.
	err = store.UpdateIndexer(ctx, id, sqlcdb.Indexer{
		Name:     "renamed",
		Priority: 10,
		Uri:      "http://there",
	})
	require.ErrorIs(t, err, storage.ErrNotFound)

	indexers, err := store.ListIndexers(ctx)
	require.NoError(t, err)
	require.Len(t, indexers, 1)
	assert.Equal(t, sqlcdb.Indexer{
		ID: id, Name: "indexer", Priority: 25, Uri: "http://here", ProwlarrID: sql.NullInt64{Int64: 1, Valid: true},
		IndexerSourceID: sql.NullInt64{Int64: sourceID, Valid: true},
		ApiKey:          sql.NullString{String: "original-key", Valid: true},
		Categories:      `{"movie":[2040],"tv":[]}`,
	}, indexers[0], "omitting source and categories preserves stored values")

	// Updating an internal indexer with an API key replaces it.
	newKey := "new-key"
	internalID, err := store.CreateIndexer(ctx, sqlcdb.Indexer{Name: "internal-key", Uri: "http://internal"})
	require.NoError(t, err)
	err = store.UpdateIndexer(ctx, internalID, sqlcdb.Indexer{
		Name:     "renamed",
		Priority: 10,
		Uri:      "http://there",
		ApiKey:   sql.NullString{String: newKey, Valid: true},
	})
	require.NoError(t, err)

	indexers, err = store.ListIndexers(ctx)
	require.NoError(t, err)
	require.Len(t, indexers, 2)
	assert.Equal(t, "new-key", indexers[1].ApiKey.String)

	// Internal rows remain deletable; sourced rows are protected.
	require.NoError(t, store.DeleteIndexer(ctx, internalID))
	require.ErrorIs(t, store.DeleteIndexer(ctx, id), storage.ErrNotFound)
	indexers, err = store.ListIndexers(ctx)
	require.NoError(t, err)
	require.Len(t, indexers, 1)
	assert.Equal(t, id, indexers[0].ID)
}
