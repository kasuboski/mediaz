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

func TestIndexerSourceStorage(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)

	sources, err := store.ListIndexerSources(ctx)
	require.NoError(t, err)
	assert.Empty(t, sources)

	source := sqlcdb.IndexerSource{
		Name:           "prowlarr",
		Implementation: "prowlarr",
		Scheme:         "http",
		Host:           "localhost",
		Port:           sql.NullInt64{Int64: 9696, Valid: true},
		ApiKey:         sql.NullString{String: "secret", Valid: true},
		Enabled:        true,
	}

	id, err := store.CreateIndexerSource(ctx, source)
	require.NoError(t, err)
	assert.NotZero(t, id)

	stored, err := store.GetIndexerSource(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, id, int64(stored.ID))
	assert.Equal(t, source.Name, stored.Name)
	assert.Equal(t, source.Implementation, stored.Implementation)
	assert.Equal(t, source.Scheme, stored.Scheme)
	assert.Equal(t, source.Host, stored.Host)
	assert.Equal(t, source.Port, stored.Port)
	assert.Equal(t, source.ApiKey, stored.ApiKey)
	assert.True(t, stored.Enabled)
	// Schema defaults apply for timestamps on new rows.
	assert.True(t, stored.CreatedAt.Valid)
	assert.True(t, stored.UpdatedAt.Valid)

	err = store.DeleteIndexerSource(ctx, id)
	require.NoError(t, err)

	sources, err = store.ListIndexerSources(ctx)
	require.NoError(t, err)
	assert.Empty(t, sources)
}

func TestGetIndexerSourceNotFound(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)

	_, err := store.GetIndexerSource(ctx, 1234)
	assert.ErrorIs(t, err, storage.ErrNotFound)
}

func TestListIndexerSourcesOrderByName(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)

	for _, name := range []string{"zeus", "alpha", "mike"} {
		_, err := store.CreateIndexerSource(ctx, sqlcdb.IndexerSource{
			Name:           name,
			Implementation: "prowlarr",
			Scheme:         "http",
			Host:           "localhost",
			Enabled:        true,
		})
		require.NoError(t, err)
	}

	sources, err := store.ListIndexerSources(ctx)
	require.NoError(t, err)
	require.Len(t, sources, 3)
	assert.Equal(t, "alpha", sources[0].Name)
	assert.Equal(t, "mike", sources[1].Name)
	assert.Equal(t, "zeus", sources[2].Name)
}

func TestListEnabledIndexerSources(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)

	enabledID, err := store.CreateIndexerSource(ctx, sqlcdb.IndexerSource{
		Name:           "enabled",
		Implementation: "prowlarr",
		Scheme:         "http",
		Host:           "localhost",
		Enabled:        true,
	})
	require.NoError(t, err)

	_, err = store.CreateIndexerSource(ctx, sqlcdb.IndexerSource{
		Name:           "disabled",
		Implementation: "prowlarr",
		Scheme:         "http",
		Host:           "localhost",
		Enabled:        false,
	})
	require.NoError(t, err)

	sources, err := store.ListEnabledIndexerSources(ctx)
	require.NoError(t, err)
	require.Len(t, sources, 1)
	assert.Equal(t, enabledID, int64(sources[0].ID))
	assert.True(t, sources[0].Enabled)
}

func TestUpdateIndexerSource(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)

	apiKey := "original-key"
	port := int64(9696)
	id, err := store.CreateIndexerSource(ctx, sqlcdb.IndexerSource{
		Name:           "prowlarr",
		Implementation: "prowlarr",
		Scheme:         "http",
		Host:           "localhost",
		Port:           sql.NullInt64{Int64: port, Valid: true},
		ApiKey:         sql.NullString{String: apiKey, Valid: true},
		Enabled:        true,
	})
	require.NoError(t, err)

	// Updating without an API key preserves the stored key.
	err = store.UpdateIndexerSource(ctx, id, sqlcdb.IndexerSource{
		Name:           "prowlarr-2",
		Implementation: "prowlarr",
		Scheme:         "https",
		Host:           "prowlarr.local",
		Port:           sql.NullInt64{},
		Enabled:        false,
	})
	require.NoError(t, err)

	stored, err := store.GetIndexerSource(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, "prowlarr-2", stored.Name)
	assert.Equal(t, "https", stored.Scheme)
	assert.Equal(t, "prowlarr.local", stored.Host)
	assert.False(t, stored.Port.Valid)
	assert.False(t, stored.Enabled)
	require.True(t, stored.ApiKey.Valid)
	assert.Equal(t, "original-key", stored.ApiKey.String)

	// Updating with an API key replaces it.
	newKey := "new-key"
	err = store.UpdateIndexerSource(ctx, id, sqlcdb.IndexerSource{
		Name:           "prowlarr-2",
		Implementation: "prowlarr",
		Scheme:         "https",
		Host:           "prowlarr.local",
		ApiKey:         sql.NullString{String: newKey, Valid: true},
		Enabled:        true,
	})
	require.NoError(t, err)

	stored, err = store.GetIndexerSource(ctx, id)
	require.NoError(t, err)
	require.True(t, stored.ApiKey.Valid)
	assert.Equal(t, "new-key", stored.ApiKey.String)
}

func TestDeleteIndexerSourceCascade(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)

	sourceA, err := store.CreateIndexerSource(ctx, sqlcdb.IndexerSource{
		Name:           "source-a",
		Implementation: "prowlarr",
		Scheme:         "http",
		Host:           "a.local",
		Enabled:        true,
	})
	require.NoError(t, err)

	sourceB, err := store.CreateIndexerSource(ctx, sqlcdb.IndexerSource{
		Name:           "source-b",
		Implementation: "prowlarr",
		Scheme:         "http",
		Host:           "b.local",
		Enabled:        true,
	})
	require.NoError(t, err)

	for _, idx := range []struct {
		name   string
		source int64
	}{
		{"a-1", sourceA},
		{"a-2", sourceA},
		{"b-1", sourceB},
	} {
		sourceID := int64(idx.source)
		_, err := store.CreateIndexer(ctx, sqlcdb.Indexer{
			Name:            idx.name,
			Priority:        25,
			Uri:             "http://" + idx.name,
			IndexerSourceID: sql.NullInt64{Int64: sourceID, Valid: true},
		})
		require.NoError(t, err)
	}

	err = store.DeleteIndexerSourceCascade(ctx, sourceA)
	require.NoError(t, err)

	sources, err := store.ListIndexerSources(ctx)
	require.NoError(t, err)
	require.Len(t, sources, 1)
	assert.Equal(t, sourceB, int64(sources[0].ID))

	indexers, err := store.ListIndexers(ctx)
	require.NoError(t, err)
	require.Len(t, indexers, 1)
	assert.Equal(t, "b-1", indexers[0].Name)
}
