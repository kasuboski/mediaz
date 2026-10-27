package manager

import (
	"context"
	"testing"

	"github.com/kasuboski/mediaz/pkg/indexer"
	indexerMocks "github.com/kasuboski/mediaz/pkg/indexer/mocks"
	"github.com/kasuboski/mediaz/pkg/prowlarr"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestSnapshotIndexerIdentityAboveSDKRange(t *testing.T) {
	const localID int64 = (1 << 32) + 1
	snapshot := newReconcileSnapshot([]sqlcdb.Indexer{{ID: localID}, {ID: 1}}, nil)
	assert.Equal(t, []int64{localID, 1}, snapshot.GetIndexerIDs())
	assert.Equal(t, []sqlcdb.Indexer{{ID: localID}, {ID: 1}}, snapshot.GetIndexers())
}

func TestSearchIndexerIdentityDoesNotAliasExternalID(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	service, indexers, sources, factory := newTestIndexerService(ctrl)
	const localID int64 = (1 << 32) + 1
	const sourceID int64 = 1 << 40
	service.indexerCache.Set(sourceID, indexerCacheEntry{Indexers: []indexer.SourceIndexer{{ID: 1, Name: "external"}}})

	// A local persistence ID must not wrap around to external Prowlarr ID 1.
	snapshot := newReconcileSnapshot([]sqlcdb.Indexer{{ID: localID}}, nil)
	releases, err := service.SearchIndexers(ctx, snapshot.GetIndexerIDs(), nil, indexer.SearchOptions{})
	require.ErrorContains(t, err, "no indexer sources")
	assert.Nil(t, releases)

	config := sqlcdb.IndexerSource{ID: sourceID, Name: "source", Enabled: true}
	sources.EXPECT().GetIndexerSource(gomock.Any(), sourceID).Return(config, nil)
	source := indexerMocks.NewMockIndexerSource(ctrl)
	factory.EXPECT().NewIndexerSource(config).Return(source, nil)
	indexers.EXPECT().ListIndexers(gomock.Any()).Return(nil, nil)
	expected := []*prowlarr.ReleaseResource{{}}
	source.EXPECT().Search(gomock.Any(), int32(1), []int32{2000}, indexer.SearchOptions{Query: "movie"}).Return(expected, nil).Times(1)
	snapshot = newReconcileSnapshot([]sqlcdb.Indexer{{ID: localID}, {ID: 1}}, nil)
	releases, err = service.SearchIndexers(ctx, snapshot.GetIndexerIDs(), []int32{2000}, indexer.SearchOptions{Query: "movie"})
	require.NoError(t, err)
	assert.Equal(t, expected, releases)
}
