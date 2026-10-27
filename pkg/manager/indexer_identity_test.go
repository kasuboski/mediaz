package manager

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/kasuboski/mediaz/pkg/indexer"
	indexerMocks "github.com/kasuboski/mediaz/pkg/indexer/mocks"
	"github.com/kasuboski/mediaz/pkg/prowlarr"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestSnapshotIndexerIdentityAboveSDKRange(t *testing.T) {
	const localID int64 = (1 << 32) + 1
	snapshot := newReconcileSnapshot([]sqlcdb.Indexer{{ID: localID}, {ID: 1}}, nil)
	assert.Equal(t, []indexer.RowID{indexer.RowID(localID), 1}, snapshot.GetIndexerIDs())
	assert.Equal(t, []sqlcdb.Indexer{{ID: localID}, {ID: 1}}, snapshot.GetIndexers())
}

func TestSearchIndexerIdentityAboveSDKRange(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	service, rows, sources, factory := newTestIndexerService(ctrl)
	const rowID indexer.RowID = (1 << 32) + 1
	const sourceID int64 = 1 << 40
	service.indexerCache.Set(sourceID, indexerCacheEntry{
		Indexers: []indexer.SourceIndexer{{ID: 1, Name: "external"}},
		RowIDs:   map[indexer.ProwlarrID]indexer.RowID{1: rowID},
	})
	// Even an exact numeric match with the external ID is not a row selection.
	_, err := service.SearchIndexers(ctx, []indexer.RowID{1}, nil, indexer.SearchOptions{})
	require.ErrorContains(t, err, "no indexer sources")
	config := sqlcdb.IndexerSource{ID: sourceID, Name: "source", Enabled: true}
	sources.EXPECT().GetIndexerSource(gomock.Any(), sourceID).Return(config, nil)
	source := indexerMocks.NewMockIndexerSource(ctrl)
	factory.EXPECT().NewIndexerSource(config).Return(source, nil)
	rows.EXPECT().ListIndexers(gomock.Any()).Return(nil, nil)
	expected := []*prowlarr.ReleaseResource{{}}
	source.EXPECT().Search(gomock.Any(), indexer.ProwlarrID(1), []int32{2000}, indexer.SearchOptions{}).Return(expected, nil)
	releases, err := service.SearchIndexers(ctx, []indexer.RowID{rowID}, []int32{2000}, indexer.SearchOptions{})
	require.NoError(t, err)
	assert.Equal(t, expected, releases)
}

func TestIndexerIdentityAcrossPersistenceAndSources(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.New(ctx, ":memory:")
	require.NoError(t, err)
	ctrl := gomock.NewController(t)
	factory := indexerMocks.NewMockFactory(ctrl)
	service := NewIndexerService(store, store, factory)
	internal := sqlcdb.Indexer{Name: "internal", Uri: "http://internal", Priority: 9}
	internalID, err := store.CreateIndexer(ctx, internal)
	require.NoError(t, err)
	require.EqualValues(t, 1, internalID)
	internal.ID = internalID
	internal.Categories = `{"movie":[],"tv":[]}`

	configs := []sqlcdb.IndexerSource{
		{Name: "first", Implementation: "prowlarr", Scheme: "http", Host: "first", Enabled: true},
		{Name: "second", Implementation: "prowlarr", Scheme: "http", Host: "second", Enabled: true},
	}
	sources := make([]*indexerMocks.MockIndexerSource, 2)
	rowIDs := make([]indexer.RowID, 2)
	for i := range configs {
		configs[i].ID, err = store.CreateIndexerSource(ctx, configs[i])
		require.NoError(t, err)
		sources[i] = indexerMocks.NewMockIndexerSource(ctrl)
		factory.EXPECT().NewIndexerSource(gomock.Any()).Return(sources[i], nil)
		// Both sources use external ID 1 and the same name, also colliding with the internal row ID.
		sources[i].EXPECT().ListIndexers(ctx).Return([]indexer.SourceIndexer{{ID: 1, Name: "external"}}, nil)
		require.NoError(t, service.RefreshIndexerSource(ctx, configs[i].ID))
		cached, ok := service.indexerCache.Get(configs[i].ID)
		require.True(t, ok)
		rowIDs[i] = cached.RowIDs[1]
		require.NotEqualValues(t, internalID, rowIDs[i])
	}
	require.NotEqual(t, rowIDs[0], rowIDs[1])
	responses, err := service.ListIndexers(ctx)
	require.NoError(t, err)
	assert.ElementsMatch(t, []IndexerResponse{
		{ID: indexer.RowID(internalID), Name: "internal", URI: "http://internal", Priority: 9, Source: "Internal"},
		{ID: rowIDs[0], Name: "external", Source: "first"},
		{ID: rowIDs[1], Name: "external", Source: "second"},
	}, responses)
	snapshotRows, err := service.listIndexersInternal(ctx)
	require.NoError(t, err)
	assert.ElementsMatch(t, []indexer.RowID{indexer.RowID(internalID), rowIDs[0], rowIDs[1]}, newReconcileSnapshot(snapshotRows, nil).GetIndexerIDs())

	_, err = service.SearchIndexers(ctx, []indexer.RowID{indexer.RowID(internalID)}, MovieCategories, indexer.SearchOptions{})
	require.ErrorContains(t, err, "no indexer sources")
	for i := range sources {
		for _, media := range []struct {
			kind     string
			fallback []int32
		}{
			{indexer.TypeMovie, MovieCategories},
			{indexer.TypeTV, TVCategories},
		} {
			factory.EXPECT().NewIndexerSource(gomock.Any()).Return(sources[i], nil)
			expected := []*prowlarr.ReleaseResource{{}}
			opts := indexer.SearchOptions{Type: &media.kind}
			sources[i].EXPECT().Search(gomock.Any(), indexer.ProwlarrID(1), media.fallback, opts).Return(expected, nil)
			got, err := service.SearchIndexers(ctx, []indexer.RowID{rowIDs[i]}, media.fallback, opts)
			require.NoError(t, err)
			assert.Equal(t, expected, got)
		}
		// Local endpoints cannot edit or delete externally managed indexers.
		_, err = service.UpdateIndexer(ctx, rowIDs[i], UpdateIndexerRequest{Name: "wrong"})
		require.Error(t, err)
		id := rowIDs[i]
		require.Error(t, service.DeleteIndexer(ctx, DeleteIndexerRequest{ID: &id}))
	}

	// Renaming at the source keeps the API/storage identity stable.
	factory.EXPECT().NewIndexerSource(gomock.Any()).Return(sources[0], nil)
	sources[0].EXPECT().ListIndexers(ctx).Return([]indexer.SourceIndexer{{ID: 1, Name: "renamed"}}, nil)
	require.NoError(t, service.RefreshIndexerSource(ctx, configs[0].ID))
	cached, _ := service.indexerCache.Get(configs[0].ID)
	assert.Equal(t, rowIDs[0], cached.RowIDs[1])
	_, err = service.AddIndexer(ctx, AddIndexerRequest{Indexer: sqlcdb.Indexer{
		Name: "injected", IndexerSourceID: sql.NullInt64{Int64: configs[0].ID, Valid: true},
		ProwlarrID: sql.NullInt64{Int64: 1, Valid: true},
	}})
	require.ErrorIs(t, err, ErrValidation)
	rows, err := store.ListIndexers(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 3)
	assert.Contains(t, rows, internal)
	for _, row := range rows {
		if row.IndexerSourceID.Valid {
			assert.Equal(t, sql.NullInt64{Int64: 1, Valid: true}, row.ProwlarrID)
		}
	}
}

func TestCategoryJSONFormatAndDefaultFallback(t *testing.T) {
	for _, tc := range []struct {
		name       string
		categories categoryIDsByType
		encoded    string
	}{
		{"empty", categoryIDsByType{}, `{"movie":[],"tv":[]}`},
		{"movie only", categoryIDsByType{Movie: []int32{2000, 2040}}, `{"movie":[2000,2040],"tv":[]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			encoded := categoriesJSON(tc.categories)
			assert.Equal(t, tc.encoded, encoded)
			var decoded categoryIDsByType
			require.NoError(t, json.Unmarshal([]byte(encoded), &decoded))
			tv := indexer.TypeTV
			assert.Equal(t, TVCategories, categoriesForSearch(decoded, TVCategories, &tv))
			if tc.name == "empty" {
				movie := indexer.TypeMovie
				assert.Equal(t, MovieCategories, categoriesForSearch(decoded, MovieCategories, &movie))
			}
		})
	}
}
