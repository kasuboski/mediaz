package manager

import (
	"context"
	"errors"
	"testing"

	"github.com/kasuboski/mediaz/pkg/indexer"
	indexerMock "github.com/kasuboski/mediaz/pkg/indexer/mocks"
	"github.com/kasuboski/mediaz/pkg/prowlarr"
	storageMocks "github.com/kasuboski/mediaz/pkg/storage/mocks"
	"github.com/oapi-codegen/nullable"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func newTestIndexerService(ctrl *gomock.Controller) (*IndexerService, *storageMocks.MockIndexerStorage, *storageMocks.MockIndexerSourceStorage, *indexerMock.MockFactory) {
	idxStorage := storageMocks.NewMockIndexerStorage(ctrl)
	srcStorage := storageMocks.NewMockIndexerSourceStorage(ctrl)
	factory := indexerMock.NewMockFactory(ctrl)
	svc := NewIndexerService(idxStorage, srcStorage, factory)
	return svc, idxStorage, srcStorage, factory
}

func TestIndexerService_AddIndexer(t *testing.T) {
	ctx := context.Background()

	t.Run("requires name", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		svc, _, _, _ := newTestIndexerService(ctrl)

		_, err := svc.AddIndexer(ctx, AddIndexerRequest{})
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrValidation)
	})

	t.Run("creates indexer", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		svc, idxStorage, _, _ := newTestIndexerService(ctrl)

		idxStorage.EXPECT().CreateIndexer(ctx, gomock.Any()).Return(int64(5), nil)

		got, err := svc.AddIndexer(ctx, AddIndexerRequest{
			Indexer: model.Indexer{Name: "test-indexer", Priority: 10, URI: "http://indexer"},
		})
		require.NoError(t, err)
		assert.Equal(t, IndexerResponse{ID: 5, Name: "test-indexer", Priority: 10, URI: "http://indexer"}, got)
	})

	t.Run("returns storage error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		svc, idxStorage, _, _ := newTestIndexerService(ctrl)

		idxStorage.EXPECT().CreateIndexer(ctx, gomock.Any()).Return(int64(0), errors.New("db error"))

		_, err := svc.AddIndexer(ctx, AddIndexerRequest{Indexer: model.Indexer{Name: "test-indexer"}})
		require.Error(t, err)
	})
}

func TestIndexerService_UpdateIndexer(t *testing.T) {
	ctx := context.Background()

	t.Run("requires name", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		svc, _, _, _ := newTestIndexerService(ctrl)

		_, err := svc.UpdateIndexer(ctx, 1, UpdateIndexerRequest{})
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrValidation)
	})

	t.Run("updates indexer", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		svc, idxStorage, _, _ := newTestIndexerService(ctrl)

		idxStorage.EXPECT().UpdateIndexer(ctx, int64(1), gomock.Any()).Return(nil)

		got, err := svc.UpdateIndexer(ctx, 1, UpdateIndexerRequest{
			Name:     "updated",
			Priority: 5,
			URI:      "http://new",
		})
		require.NoError(t, err)
		assert.Equal(t, IndexerResponse{ID: 1, Name: "updated", Priority: 5, URI: "http://new"}, got)
	})

	t.Run("returns storage error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		svc, idxStorage, _, _ := newTestIndexerService(ctrl)

		idxStorage.EXPECT().UpdateIndexer(ctx, int64(1), gomock.Any()).Return(errors.New("db error"))

		_, err := svc.UpdateIndexer(ctx, 1, UpdateIndexerRequest{Name: "updated"})
		require.Error(t, err)
	})
}

func TestIndexerService_DeleteIndexer(t *testing.T) {
	ctx := context.Background()

	t.Run("requires id", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		svc, _, _, _ := newTestIndexerService(ctrl)

		err := svc.DeleteIndexer(ctx, DeleteIndexerRequest{ID: nil})
		require.Error(t, err)
	})

	t.Run("deletes indexer", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		svc, idxStorage, _, _ := newTestIndexerService(ctrl)

		id := 3
		idxStorage.EXPECT().DeleteIndexer(ctx, int64(3)).Return(nil)

		err := svc.DeleteIndexer(ctx, DeleteIndexerRequest{ID: &id})
		require.NoError(t, err)
	})
}

func TestIndexerService_ListIndexers(t *testing.T) {
	ctx := context.Background()

	t.Run("returns internal indexers from db", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		svc, idxStorage, _, _ := newTestIndexerService(ctrl)

		idxStorage.EXPECT().ListIndexers(ctx).Return([]*model.Indexer{
			{ID: 1, Name: "db-indexer", Priority: 1, URI: "http://db", IndexerSourceID: nil},
		}, nil)

		got, err := svc.ListIndexers(ctx)
		require.NoError(t, err)
		assert.Equal(t, []IndexerResponse{
			{ID: 1, Name: "db-indexer", Priority: 1, URI: "http://db", Source: "Internal"},
		}, got)
	})

	t.Run("returns cached indexers from sources", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		svc, idxStorage, srcStorage, factory := newTestIndexerService(ctrl)

		src := indexerMock.NewMockIndexerSource(ctrl)
		srcStorage.EXPECT().GetIndexerSource(ctx, int64(10)).Return(model.IndexerSource{
			ID: 10, Name: "prowlarr", Scheme: "http", Host: "prowlarr-host", Enabled: true,
		}, nil)
		factory.EXPECT().NewIndexerSource(gomock.Any()).Return(src, nil)
		src.EXPECT().ListIndexers(ctx).Return([]indexer.SourceIndexer{{ID: 99, Name: "cached-indexer", Priority: 3}}, nil)
		require.NoError(t, svc.RefreshIndexerSource(ctx, 10))

		idxStorage.EXPECT().ListIndexers(ctx).Return(nil, nil)

		got, err := svc.ListIndexers(ctx)
		require.NoError(t, err)
		assert.Equal(t, []IndexerResponse{
			{ID: 99, Name: "cached-indexer", Priority: 3, Source: "prowlarr"},
		}, got)
	})

	t.Run("skips db indexers that belong to a source", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		svc, idxStorage, _, _ := newTestIndexerService(ctrl)

		srcID := int32(1)
		idxStorage.EXPECT().ListIndexers(ctx).Return([]*model.Indexer{
			{ID: 1, Name: "source-indexer", IndexerSourceID: &srcID},
			{ID: 2, Name: "internal-indexer", Priority: 5, URI: "http://internal", IndexerSourceID: nil},
		}, nil)

		got, err := svc.ListIndexers(ctx)
		require.NoError(t, err)
		assert.Equal(t, []IndexerResponse{
			{ID: 2, Name: "internal-indexer", Priority: 5, URI: "http://internal", Source: "Internal"},
		}, got)
	})
}

func TestIndexerService_CreateIndexerSource(t *testing.T) {
	ctx := context.Background()

	t.Run("creates disabled source", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		svc, _, srcStorage, _ := newTestIndexerService(ctrl)

		srcStorage.EXPECT().CreateIndexerSource(ctx, gomock.Any()).Return(int64(7), nil)

		got, err := svc.CreateIndexerSource(ctx, AddIndexerSourceRequest{
			Name:           "prowlarr",
			Implementation: "prowlarr",
			Scheme:         "http",
			Host:           "localhost",
			Enabled:        false,
		})
		require.NoError(t, err)
		assert.Equal(t, IndexerSourceResponse{
			ID:             7,
			Name:           "prowlarr",
			Implementation: "prowlarr",
			Scheme:         "http",
			Host:           "localhost",
			Enabled:        false,
		}, got)
	})

	t.Run("refreshes cache when enabled", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		svc, _, srcStorage, factory := newTestIndexerService(ctrl)

		src := indexerMock.NewMockIndexerSource(ctrl)
		srcStorage.EXPECT().CreateIndexerSource(ctx, gomock.Any()).Return(int64(1), nil)
		srcStorage.EXPECT().GetIndexerSource(ctx, int64(1)).Return(model.IndexerSource{
			ID:             1,
			Name:           "prowlarr",
			Implementation: "prowlarr",
			Scheme:         "http",
			Host:           "localhost",
			Enabled:        true,
		}, nil)
		factory.EXPECT().NewIndexerSource(gomock.Any()).Return(src, nil)
		src.EXPECT().ListIndexers(ctx).Return([]indexer.SourceIndexer{
			{ID: 1, Name: "indexer-1"},
		}, nil)

		got, err := svc.CreateIndexerSource(ctx, AddIndexerSourceRequest{
			Name:           "prowlarr",
			Implementation: "prowlarr",
			Scheme:         "http",
			Host:           "localhost",
			Enabled:        true,
		})
		require.NoError(t, err)
		assert.Equal(t, IndexerSourceResponse{
			ID:             1,
			Name:           "prowlarr",
			Implementation: "prowlarr",
			Scheme:         "http",
			Host:           "localhost",
			Enabled:        true,
		}, got)

		cached, ok := svc.indexerCache.Get(int64(1))
		require.True(t, ok)
		assert.Equal(t, indexerCacheEntry{
			Indexers:   []indexer.SourceIndexer{{ID: 1, Name: "indexer-1"}},
			SourceName: "prowlarr",
			SourceURI:  "http://localhost",
		}, cached)
	})

	t.Run("returns storage error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		svc, _, srcStorage, _ := newTestIndexerService(ctrl)

		srcStorage.EXPECT().CreateIndexerSource(ctx, gomock.Any()).Return(int64(0), errors.New("db error"))

		_, err := svc.CreateIndexerSource(ctx, AddIndexerSourceRequest{Name: "prowlarr"})
		require.Error(t, err)
	})
}

func TestIndexerService_ListIndexerSources(t *testing.T) {
	ctx := context.Background()

	t.Run("returns sources", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		svc, _, srcStorage, _ := newTestIndexerService(ctrl)

		srcStorage.EXPECT().ListIndexerSources(ctx).Return([]*model.IndexerSource{
			{ID: 1, Name: "prowlarr", Implementation: "prowlarr", Scheme: "http", Host: "localhost", Enabled: true},
			{ID: 2, Name: "jackett", Implementation: "jackett", Scheme: "https", Host: "jackett.example.com"},
		}, nil)

		got, err := svc.ListIndexerSources(ctx)
		require.NoError(t, err)
		assert.Equal(t, []IndexerSourceResponse{
			{ID: 1, Name: "prowlarr", Implementation: "prowlarr", Scheme: "http", Host: "localhost", Enabled: true},
			{ID: 2, Name: "jackett", Implementation: "jackett", Scheme: "https", Host: "jackett.example.com"},
		}, got)
	})

	t.Run("returns storage error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		svc, _, srcStorage, _ := newTestIndexerService(ctrl)

		srcStorage.EXPECT().ListIndexerSources(ctx).Return(nil, errors.New("db error"))

		_, err := svc.ListIndexerSources(ctx)
		require.Error(t, err)
	})
}

func TestIndexerService_GetIndexerSource(t *testing.T) {
	ctx := context.Background()

	t.Run("returns source", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		svc, _, srcStorage, _ := newTestIndexerService(ctrl)

		srcStorage.EXPECT().GetIndexerSource(ctx, int64(1)).Return(model.IndexerSource{
			ID:             1,
			Name:           "prowlarr",
			Implementation: "prowlarr",
			Scheme:         "http",
			Host:           "localhost",
			Enabled:        true,
		}, nil)

		got, err := svc.GetIndexerSource(ctx, 1)
		require.NoError(t, err)
		assert.Equal(t, IndexerSourceResponse{
			ID:             1,
			Name:           "prowlarr",
			Implementation: "prowlarr",
			Scheme:         "http",
			Host:           "localhost",
			Enabled:        true,
		}, got)
	})

	t.Run("returns error when not found", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		svc, _, srcStorage, _ := newTestIndexerService(ctrl)

		srcStorage.EXPECT().GetIndexerSource(ctx, int64(99)).Return(model.IndexerSource{}, errors.New("not found"))

		_, err := svc.GetIndexerSource(ctx, 99)
		require.Error(t, err)
	})
}

func TestIndexerService_UpdateIndexerSource(t *testing.T) {
	ctx := context.Background()

	t.Run("updates source with provided api key", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		svc, _, srcStorage, _ := newTestIndexerService(ctrl)

		existingKey := "old-key"
		newKey := "new-key"
		srcStorage.EXPECT().GetIndexerSource(ctx, int64(1)).Return(model.IndexerSource{
			ID:     1,
			APIKey: &existingKey,
		}, nil)
		srcStorage.EXPECT().UpdateIndexerSource(ctx, int64(1), gomock.Any()).Return(nil)

		got, err := svc.UpdateIndexerSource(ctx, 1, UpdateIndexerSourceRequest{
			Name:           "updated",
			Implementation: "prowlarr",
			Scheme:         "https",
			Host:           "new-host",
			APIKey:         &newKey,
		})
		require.NoError(t, err)
		assert.Equal(t, IndexerSourceResponse{
			ID:             1,
			Name:           "updated",
			Implementation: "prowlarr",
			Scheme:         "https",
			Host:           "new-host",
		}, got)
	})

	t.Run("preserves existing api key when empty string provided", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		svc, _, srcStorage, _ := newTestIndexerService(ctrl)

		existingKey := "existing-key"
		srcStorage.EXPECT().GetIndexerSource(ctx, int64(1)).Return(model.IndexerSource{
			ID:     1,
			APIKey: &existingKey,
		}, nil)
		srcStorage.EXPECT().UpdateIndexerSource(ctx, int64(1), gomock.Any()).DoAndReturn(
			func(_ context.Context, _ int64, src model.IndexerSource) error {
				assert.Equal(t, &existingKey, src.APIKey)
				return nil
			},
		)

		emptyKey := ""
		_, err := svc.UpdateIndexerSource(ctx, 1, UpdateIndexerSourceRequest{
			Name:   "updated",
			APIKey: &emptyKey,
		})
		require.NoError(t, err)
	})

	t.Run("preserves existing api key when nil provided", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		svc, _, srcStorage, _ := newTestIndexerService(ctrl)

		existingKey := "existing-key"
		srcStorage.EXPECT().GetIndexerSource(ctx, int64(1)).Return(model.IndexerSource{
			ID:     1,
			APIKey: &existingKey,
		}, nil)
		srcStorage.EXPECT().UpdateIndexerSource(ctx, int64(1), gomock.Any()).DoAndReturn(
			func(_ context.Context, _ int64, src model.IndexerSource) error {
				assert.Equal(t, &existingKey, src.APIKey)
				return nil
			},
		)

		_, err := svc.UpdateIndexerSource(ctx, 1, UpdateIndexerSourceRequest{
			Name:   "updated",
			APIKey: nil,
		})
		require.NoError(t, err)
	})
}

func TestIndexerService_DeleteIndexerSource(t *testing.T) {
	ctx := context.Background()

	t.Run("cascades to child indexers and clears cache", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		svc, idxStorage, srcStorage, factory := newTestIndexerService(ctrl)

		src := indexerMock.NewMockIndexerSource(ctrl)
		srcStorage.EXPECT().GetIndexerSource(ctx, int64(1)).Return(model.IndexerSource{
			ID: 1, Name: "prowlarr", Scheme: "http", Host: "prowlarr-host", Enabled: true,
		}, nil)
		factory.EXPECT().NewIndexerSource(gomock.Any()).Return(src, nil)
		src.EXPECT().ListIndexers(ctx).Return(nil, nil)
		require.NoError(t, svc.RefreshIndexerSource(ctx, 1))

		idxStorage.EXPECT().ListIndexers(ctx, gomock.Any()).Return([]*model.Indexer{
			{ID: 10}, {ID: 11},
		}, nil)
		idxStorage.EXPECT().DeleteIndexer(ctx, int64(10)).Return(nil)
		idxStorage.EXPECT().DeleteIndexer(ctx, int64(11)).Return(nil)
		srcStorage.EXPECT().DeleteIndexerSource(ctx, int64(1)).Return(nil)

		err := svc.DeleteIndexerSource(ctx, 1)
		require.NoError(t, err)

		_, ok := svc.indexerCache.Get(int64(1))
		assert.False(t, ok)
	})

	t.Run("returns error if listing child indexers fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		svc, idxStorage, _, _ := newTestIndexerService(ctrl)

		idxStorage.EXPECT().ListIndexers(ctx, gomock.Any()).Return(nil, errors.New("db error"))

		err := svc.DeleteIndexerSource(ctx, 1)
		require.Error(t, err)
	})
}

func TestIndexerService_TestIndexerSource(t *testing.T) {
	ctx := context.Background()

	t.Run("succeeds when source lists indexers", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		svc, _, _, factory := newTestIndexerService(ctrl)

		src := indexerMock.NewMockIndexerSource(ctrl)
		factory.EXPECT().NewIndexerSource(gomock.Any()).Return(src, nil)
		src.EXPECT().ListIndexers(ctx).Return([]indexer.SourceIndexer{{ID: 1, Name: "idx-1"}}, nil)

		err := svc.TestIndexerSource(ctx, AddIndexerSourceRequest{
			Name:           "prowlarr",
			Implementation: "prowlarr",
			Scheme:         "http",
			Host:           "localhost",
		})
		require.NoError(t, err)
	})

	t.Run("returns error when factory fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		svc, _, _, factory := newTestIndexerService(ctrl)

		factory.EXPECT().NewIndexerSource(gomock.Any()).Return(nil, errors.New("unsupported implementation"))

		err := svc.TestIndexerSource(ctx, AddIndexerSourceRequest{
			Name:           "unknown",
			Implementation: "unknown",
		})
		require.Error(t, err)
	})

	t.Run("returns error when connection fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		svc, _, _, factory := newTestIndexerService(ctrl)

		src := indexerMock.NewMockIndexerSource(ctrl)
		factory.EXPECT().NewIndexerSource(gomock.Any()).Return(src, nil)
		src.EXPECT().ListIndexers(ctx).Return(nil, errors.New("connection refused"))

		err := svc.TestIndexerSource(ctx, AddIndexerSourceRequest{
			Name:           "prowlarr",
			Implementation: "prowlarr",
			Scheme:         "http",
			Host:           "unreachable",
		})
		require.Error(t, err)
	})
}

func TestIndexerService_RefreshIndexerSource(t *testing.T) {
	ctx := context.Background()

	t.Run("skips disabled source without touching cache", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		svc, _, srcStorage, _ := newTestIndexerService(ctrl)

		srcStorage.EXPECT().GetIndexerSource(ctx, int64(1)).Return(model.IndexerSource{
			ID:      1,
			Enabled: false,
		}, nil)

		err := svc.RefreshIndexerSource(ctx, 1)
		require.NoError(t, err)

		_, ok := svc.indexerCache.Get(int64(1))
		assert.False(t, ok)
	})

	t.Run("populates cache for enabled source", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		svc, _, srcStorage, factory := newTestIndexerService(ctrl)

		src := indexerMock.NewMockIndexerSource(ctrl)
		srcStorage.EXPECT().GetIndexerSource(ctx, int64(1)).Return(model.IndexerSource{
			ID:      1,
			Name:    "prowlarr",
			Scheme:  "http",
			Host:    "localhost",
			Enabled: true,
		}, nil)
		factory.EXPECT().NewIndexerSource(gomock.Any()).Return(src, nil)
		src.EXPECT().ListIndexers(ctx).Return([]indexer.SourceIndexer{
			{ID: 1, Name: "indexer-1"},
			{ID: 2, Name: "indexer-2"},
		}, nil)

		err := svc.RefreshIndexerSource(ctx, 1)
		require.NoError(t, err)

		got, ok := svc.indexerCache.Get(int64(1))
		require.True(t, ok)
		assert.Equal(t, indexerCacheEntry{
			Indexers:   []indexer.SourceIndexer{{ID: 1, Name: "indexer-1"}, {ID: 2, Name: "indexer-2"}},
			SourceName: "prowlarr",
			SourceURI:  "http://localhost",
		}, got)
	})

	t.Run("returns error when list indexers fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		svc, _, srcStorage, factory := newTestIndexerService(ctrl)

		src := indexerMock.NewMockIndexerSource(ctrl)
		srcStorage.EXPECT().GetIndexerSource(ctx, int64(1)).Return(model.IndexerSource{
			ID:      1,
			Enabled: true,
		}, nil)
		factory.EXPECT().NewIndexerSource(gomock.Any()).Return(src, nil)
		src.EXPECT().ListIndexers(ctx).Return(nil, errors.New("upstream error"))

		err := svc.RefreshIndexerSource(ctx, 1)
		require.Error(t, err)
	})
}

func TestIndexerService_RefreshAllIndexerSources(t *testing.T) {
	ctx := context.Background()

	t.Run("refreshes all enabled sources", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		svc, _, srcStorage, factory := newTestIndexerService(ctrl)

		src := indexerMock.NewMockIndexerSource(ctrl)
		srcStorage.EXPECT().ListIndexerSources(ctx, gomock.Any()).Return([]*model.IndexerSource{
			{ID: 1, Name: "src1", Scheme: "http", Host: "host1", Enabled: true},
			{ID: 2, Name: "src2", Scheme: "http", Host: "host2", Enabled: true},
		}, nil)
		srcStorage.EXPECT().GetIndexerSource(ctx, int64(1)).Return(model.IndexerSource{
			ID: 1, Name: "src1", Scheme: "http", Host: "host1", Enabled: true,
		}, nil)
		srcStorage.EXPECT().GetIndexerSource(ctx, int64(2)).Return(model.IndexerSource{
			ID: 2, Name: "src2", Scheme: "http", Host: "host2", Enabled: true,
		}, nil)
		factory.EXPECT().NewIndexerSource(gomock.Any()).Return(src, nil).Times(2)
		src.EXPECT().ListIndexers(ctx).Return([]indexer.SourceIndexer{{ID: 1, Name: "idx-1"}}, nil).Times(2)

		err := svc.RefreshAllIndexerSources(ctx)
		require.NoError(t, err)

		e1, ok1 := svc.indexerCache.Get(int64(1))
		e2, ok2 := svc.indexerCache.Get(int64(2))
		require.True(t, ok1)
		require.True(t, ok2)
		assert.Equal(t, indexerCacheEntry{
			Indexers:   []indexer.SourceIndexer{{ID: 1, Name: "idx-1"}},
			SourceName: "src1",
			SourceURI:  "http://host1",
		}, e1)
		assert.Equal(t, indexerCacheEntry{
			Indexers:   []indexer.SourceIndexer{{ID: 1, Name: "idx-1"}},
			SourceName: "src2",
			SourceURI:  "http://host2",
		}, e2)
	})

	t.Run("accumulates errors across sources", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		svc, _, srcStorage, factory := newTestIndexerService(ctrl)

		src := indexerMock.NewMockIndexerSource(ctrl)
		srcStorage.EXPECT().ListIndexerSources(ctx, gomock.Any()).Return([]*model.IndexerSource{
			{ID: 1, Enabled: true},
			{ID: 2, Enabled: true},
		}, nil)
		srcStorage.EXPECT().GetIndexerSource(ctx, int64(1)).Return(model.IndexerSource{ID: 1, Enabled: true}, nil)
		srcStorage.EXPECT().GetIndexerSource(ctx, int64(2)).Return(model.IndexerSource{ID: 2, Enabled: true}, nil)
		factory.EXPECT().NewIndexerSource(gomock.Any()).Return(src, nil).Times(2)
		src.EXPECT().ListIndexers(ctx).Return(nil, errors.New("upstream down")).Times(2)

		err := svc.RefreshAllIndexerSources(ctx)
		require.Error(t, err)
	})
}

func TestIndexerService_SearchIndexers(t *testing.T) {
	ctx := context.Background()

	t.Run("returns error when no sources found for requested indexers", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		svc, _, _, _ := newTestIndexerService(ctrl)

		_, err := svc.SearchIndexers(ctx, []int32{1, 2}, nil, indexer.SearchOptions{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no indexer sources found")
	})

	t.Run("searches cached source and returns releases", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		svc, _, srcStorage, factory := newTestIndexerService(ctrl)

		src := indexerMock.NewMockIndexerSource(ctrl)
		srcStorage.EXPECT().GetIndexerSource(ctx, int64(1)).Return(model.IndexerSource{
			ID: 1, Name: "prowlarr", Scheme: "http", Host: "prowlarr-host", Enabled: true,
		}, nil).Times(2)
		factory.EXPECT().NewIndexerSource(gomock.Any()).Return(src, nil).Times(2)
		src.EXPECT().ListIndexers(ctx).Return([]indexer.SourceIndexer{{ID: 1, Name: "indexer-1"}}, nil)
		require.NoError(t, svc.RefreshIndexerSource(ctx, 1))

		want := []*prowlarr.ReleaseResource{{Title: nullable.NewNullableWithValue("Test Movie")}}
		src.EXPECT().Search(ctx, int32(1), gomock.Any(), gomock.Any()).Return(want, nil)

		got, err := svc.SearchIndexers(ctx, []int32{1}, nil, indexer.SearchOptions{Query: "test movie"})
		require.NoError(t, err)
		assert.Equal(t, want, got)
	})
}
