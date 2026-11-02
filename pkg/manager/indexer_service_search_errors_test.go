package manager

import (
	"context"
	"errors"
	"testing"

	"github.com/kasuboski/mediaz/pkg/indexer"
	indexerMock "github.com/kasuboski/mediaz/pkg/indexer/mocks"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/oapi-codegen/nullable"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// Search errors are surfaced without discarding releases returned by other
// indexers, including when failures and successes share a source.
func TestSearchIndexersPartialResultsAndErrors(t *testing.T) {
	for _, tc := range []struct {
		name       string
		sameSource bool
	}{
		{name: "same source", sameSource: true},
		{name: "cross source"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			ctrl := gomock.NewController(t)
			svc, idxStorage, srcStorage, factory := newTestIndexerService(ctrl)
			count := int64(2)
			if tc.sameSource {
				count = 1
			}
			sources := make([]*indexerMock.MockIndexerSource, count)
			for i := int64(1); i <= count; i++ {
				source := indexerMock.NewMockIndexerSource(ctrl)
				sources[i-1] = source
				srcStorage.EXPECT().GetIndexerSource(ctx, i).Return(sqlcdb.IndexerSource{ID: i, Enabled: true}, nil)
				factory.EXPECT().NewIndexerSource(sqlcdb.IndexerSource{ID: i, Enabled: true}).Return(source, nil)
				id := indexer.SourceID(i)
				source.EXPECT().ListIndexers(ctx).Return([]indexer.SourceIndexer{{ID: id, Name: "indexer"}}, nil)
				idxStorage.EXPECT().CreateIndexer(ctx, gomock.Any()).Return(i, nil)
				require.NoError(t, svc.RefreshIndexerSource(ctx, i))
			}
			// Two indexers on one source, or one indexer on each of two sources.
			if tc.sameSource {
				// Add a second cached indexer to the first source for the same-source case.
				entry, _ := svc.indexerCache.Get(1)
				entry.Indexers = append(entry.Indexers, indexer.SourceIndexer{ID: indexer.SourceID(2), Name: "second"})
				entry.RowIDs[indexer.SourceID(2)] = 2
				svc.indexerCache.Set(1, entry)
				count = 1
			}
			for i := int64(1); i <= count; i++ {
				idxStorage.EXPECT().ListIndexers(gomock.Any()).Return([]sqlcdb.Indexer{}, nil)
				srcStorage.EXPECT().GetIndexerSource(gomock.Any(), i).Return(sqlcdb.IndexerSource{ID: i, Enabled: true}, nil)
				factory.EXPECT().NewIndexerSource(sqlcdb.IndexerSource{ID: i, Enabled: true}).Return(sources[i-1], nil)
			}
			good := &indexer.Release{Title: nullable.NewNullableWithValue("kept")}
			partial := &indexer.Release{Title: nullable.NewNullableWithValue("partial")}
			// In same-source case indexer 1 succeeds and 2 fails; cross-source
			// case source 1 succeeds and source 2 fails.
			if tc.sameSource {
				sources[0].EXPECT().Search(gomock.Any(), indexer.SourceID(1), gomock.Any(), gomock.Any()).Return([]*indexer.Release{good}, nil)
				sources[0].EXPECT().Search(gomock.Any(), indexer.SourceID(2), gomock.Any(), gomock.Any()).Return([]*indexer.Release{partial}, errors.New("offline"))
			} else {
				sources[0].EXPECT().Search(gomock.Any(), indexer.SourceID(1), gomock.Any(), gomock.Any()).Return([]*indexer.Release{good}, nil)
				sources[1].EXPECT().Search(gomock.Any(), indexer.SourceID(2), gomock.Any(), gomock.Any()).Return([]*indexer.Release{partial}, errors.New("offline"))
			}
			got, err := svc.SearchIndexers(ctx, []indexer.RowID{1, 2}, nil, indexer.SearchOptions{})
			require.Error(t, err)
			require.Contains(t, err.Error(), "indexer")
			require.Contains(t, err.Error(), "source")
			require.Contains(t, err.Error(), "offline")
			require.ElementsMatch(t, []*indexer.Release{good, partial}, got)
			require.Contains(t, err.Error(), "indexer 2")
			if tc.sameSource {
				require.Contains(t, err.Error(), "source 1")
			} else {
				require.Contains(t, err.Error(), "source 2")
			}
		})
	}
}
