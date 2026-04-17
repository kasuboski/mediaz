package manager

import (
	"context"
	"errors"
	"testing"

	mockLibrary "github.com/kasuboski/mediaz/pkg/library/mocks"
	"github.com/kasuboski/mediaz/pkg/ptr"
	"github.com/kasuboski/mediaz/pkg/storage"
	storageMocks "github.com/kasuboski/mediaz/pkg/storage/mocks"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/kasuboski/mediaz/pkg/tmdb"
	tmdbMocks "github.com/kasuboski/mediaz/pkg/tmdb/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// mockMetadataProvider is a test double for SeriesMetadataProvider
type mockMetadataProvider struct {
	metadata *model.SeriesMetadata
	err      error
}

func (m *mockMetadataProvider) GetSeriesMetadata(ctx context.Context, tmdbID int) (*model.SeriesMetadata, error) {
	return m.metadata, m.err
}

func newTestSeriesService(ctrl *gomock.Controller) (*SeriesService, *storageMocks.MockStorage, *tmdbMocks.MockITmdb, *mockLibrary.MockLibrary) {
	store := storageMocks.NewMockStorage(ctrl)
	tmdbClient := tmdbMocks.NewMockITmdb(ctrl)
	lib := mockLibrary.NewMockLibrary(ctrl)
	qualityService := NewQualityService(store)
	metadataProvider := &mockMetadataProvider{}

	svc := NewSeriesService(tmdbClient, lib, store, store, qualityService, metadataProvider)
	return svc, store, tmdbClient, lib
}

func TestSeriesService_UpdateSeriesMonitored(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	svc, store, _, _ := newTestSeriesService(ctrl)

	series := &storage.Series{
		Series: model.Series{
			ID:        1,
			Monitored: 1,
		},
	}

	store.EXPECT().UpdateSeries(ctx, gomock.Any(), gomock.Any()).Return(nil)
	store.EXPECT().GetSeries(ctx, gomock.Any()).Return(series, nil)

	result, err := svc.UpdateSeriesMonitored(ctx, 1, true)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, int32(1), result.ID)
}

func TestSeriesService_UpdateSeriesMonitored_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	svc, store, _, _ := newTestSeriesService(ctrl)

	store.EXPECT().UpdateSeries(ctx, gomock.Any(), gomock.Any()).Return(errors.New("db error"))

	result, err := svc.UpdateSeriesMonitored(ctx, 1, true)
	require.Error(t, err)
	assert.Nil(t, result)
}

func TestSeriesService_DeleteSeries(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	svc, store, _, _ := newTestSeriesService(ctrl)

	path := "Test Series"
	series := &storage.Series{
		Series: model.Series{
			ID:   1,
			Path: &path,
		},
	}

	store.EXPECT().GetSeries(ctx, gomock.Any()).Return(series, nil)
	store.EXPECT().ListSeasons(ctx, gomock.Any()).Return([]*storage.Season{}, nil)
	store.EXPECT().DeleteSeries(ctx, int64(1)).Return(nil)

	err := svc.DeleteSeries(ctx, 1, false)
	require.NoError(t, err)
}

func TestSeriesService_DeleteSeries_WithDirectory(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	svc, store, _, lib := newTestSeriesService(ctrl)

	path := "Test Series"
	series := &storage.Series{
		Series: model.Series{
			ID:   1,
			Path: &path,
		},
	}

	store.EXPECT().GetSeries(ctx, gomock.Any()).Return(series, nil)
	store.EXPECT().ListSeasons(ctx, gomock.Any()).Return([]*storage.Season{}, nil)
	lib.EXPECT().DeleteSeriesDirectory(ctx, path).Return(nil)
	store.EXPECT().DeleteSeries(ctx, int64(1)).Return(nil)

	err := svc.DeleteSeries(ctx, 1, true)
	require.NoError(t, err)
}

func TestSeriesService_DeleteSeries_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	svc, store, _, _ := newTestSeriesService(ctrl)

	store.EXPECT().GetSeries(ctx, gomock.Any()).Return(nil, storage.ErrNotFound)

	err := svc.DeleteSeries(ctx, 1, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get series")
}

func TestSeriesService_DeleteSeries_NilPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	svc, store, _, _ := newTestSeriesService(ctrl)

	series := &storage.Series{
		Series: model.Series{
			ID:   1,
			Path: nil,
		},
	}

	store.EXPECT().GetSeries(ctx, gomock.Any()).Return(series, nil)
	store.EXPECT().ListSeasons(ctx, gomock.Any()).Return([]*storage.Season{}, nil)

	err := svc.DeleteSeries(ctx, 1, true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "series path is nil")
}

func TestSeriesService_DeleteSeries_WithEpisodeFiles(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	svc, store, _, lib := newTestSeriesService(ctrl)

	path := "Test Series"
	series := &storage.Series{
		Series: model.Series{
			ID:   1,
			Path: &path,
		},
	}

	episodeFileID := int32(10)
	season := &storage.Season{
		Season: model.Season{
			ID:       1,
			SeriesID: 1,
		},
	}

	episode := &storage.Episode{
		Episode: model.Episode{
			ID:            1,
			SeasonID:      1,
			EpisodeFileID: &episodeFileID,
		},
	}

	store.EXPECT().GetSeries(ctx, gomock.Any()).Return(series, nil)
	store.EXPECT().ListSeasons(ctx, gomock.Any()).Return([]*storage.Season{season}, nil)
	store.EXPECT().ListEpisodes(ctx, gomock.Any()).Return([]*storage.Episode{episode}, nil)
	store.EXPECT().DeleteEpisodeFile(ctx, int64(episodeFileID)).Return(nil)
	lib.EXPECT().DeleteSeriesDirectory(ctx, path).Return(nil)
	store.EXPECT().DeleteSeries(ctx, int64(1)).Return(nil)

	err := svc.DeleteSeries(ctx, 1, true)
	require.NoError(t, err)
}

func TestSeriesService_DeleteSeries_WithEpisodeFiles_NoDeleteDirectory(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	svc, store, _, _ := newTestSeriesService(ctrl)

	path := "Test Series"
	series := &storage.Series{
		Series: model.Series{
			ID:   1,
			Path: &path,
		},
	}

	season := &storage.Season{
		Season: model.Season{
			ID:       1,
			SeriesID: 1,
		},
	}

	// When deleteDirectory is false, episode files should NOT be deleted.
	// ListSeasons is still called but episodes loop is skipped entirely.
	store.EXPECT().GetSeries(ctx, gomock.Any()).Return(series, nil)
	store.EXPECT().ListSeasons(ctx, gomock.Any()).Return([]*storage.Season{season}, nil)
	// No ListEpisodes, DeleteEpisodeFile, or DeleteSeriesDirectory calls expected
	store.EXPECT().DeleteSeries(ctx, int64(1)).Return(nil)

	err := svc.DeleteSeries(ctx, 1, false)
	require.NoError(t, err)
}

func TestSeriesService_ListShowsInLibrary(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	svc, store, _, _ := newTestSeriesService(ctrl)

	metadataID := int32(1)
	series := []*storage.Series{
		{
			Series: model.Series{
				ID:               1,
				SeriesMetadataID: &metadataID,
				Path:             ptr.To("Test Show"),
			},
			State: storage.SeriesStateMissing,
		},
	}

	posterPath := "poster.jpg"
	metadata := &model.SeriesMetadata{
		ID:         1,
		TmdbID:     123,
		Title:      "Test Show",
		PosterPath: &posterPath,
	}

	store.EXPECT().ListSeries(ctx).Return(series, nil)
	store.EXPECT().GetSeriesMetadata(ctx, gomock.Any()).Return(metadata, nil)

	shows, err := svc.ListShowsInLibrary(ctx)
	require.NoError(t, err)
	require.Len(t, shows, 1)
	assert.Equal(t, "Test Show", shows[0].Title)
	assert.Equal(t, int32(123), shows[0].TMDBID)
	assert.Equal(t, string(storage.SeriesStateMissing), shows[0].State)
}

func TestSeriesService_ListShowsInLibrary_SkipsNoMetadata(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	svc, store, _, _ := newTestSeriesService(ctrl)

	series := []*storage.Series{
		{
			Series: model.Series{
				ID:               1,
				SeriesMetadataID: nil,
				Path:             ptr.To("Test Show"),
			},
		},
	}

	store.EXPECT().ListSeries(ctx).Return(series, nil)

	shows, err := svc.ListShowsInLibrary(ctx)
	require.NoError(t, err)
	assert.Empty(t, shows)
}

func TestSeriesService_GetSeriesDetails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	svc, _, tmdbClient, _ := newTestSeriesService(ctrl)

	tmdbClient.EXPECT().GetSeriesDetails(ctx, 123).Return(&tmdb.SeriesDetails{
		ID:              123,
		Name:            "Test Series",
		NumberOfSeasons: 2,
		FirstAirDate:    "2023-01-01",
	}, nil)

	result, err := svc.GetSeriesDetails(ctx, 123)
	require.NoError(t, err)
	assert.Equal(t, int32(123), result.TmdbID)
	assert.Equal(t, "Test Series", result.Title)
}

func TestSeriesService_SearchTV_EmptyQuery(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	svc, _, _, _ := newTestSeriesService(ctrl)

	_, err := svc.SearchTV(ctx, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "query is empty")
}

func TestNewSeriesService(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := storageMocks.NewMockStorage(ctrl)
	tmdbClient := tmdbMocks.NewMockITmdb(ctrl)
	lib := mockLibrary.NewMockLibrary(ctrl)
	qualityService := NewQualityService(store)
	metadataProvider := &mockMetadataProvider{}

	svc := NewSeriesService(tmdbClient, lib, store, store, qualityService, metadataProvider)
	require.NotNil(t, svc)
	assert.Equal(t, tmdbClient, svc.tmdb)
	assert.Equal(t, lib, svc.library)
	assert.NotNil(t, svc.qualityService)
	assert.Equal(t, metadataProvider, svc.metadataProvider)
}

func TestSeriesService_buildTVDetailResult(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc, _, _, _ := newTestSeriesService(ctrl)
	overview := "A great show"
	metadata := &model.SeriesMetadata{
		TmdbID:       123,
		Title:        "Test Series",
		SeasonCount:  3,
		EpisodeCount: 30,
		Overview:     &overview,
	}

	result := svc.buildTVDetailResult(metadata, &tmdb.SeriesDetailsResponse{}, nil, nil)
	assert.Equal(t, int32(123), result.TMDBID)
	assert.Equal(t, "Test Series", result.Title)
	assert.Equal(t, int32(3), result.SeasonCount)
	assert.Equal(t, int32(30), result.EpisodeCount)
	assert.Equal(t, "Not In Library", result.LibraryStatus)
	assert.Equal(t, "A great show", result.Overview)
}

func TestSeriesService_buildTVDetailResult_WithSeries(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc, _, _, _ := newTestSeriesService(ctrl)
	path := "/tv/show"
	monitored := int32(1)
	metadata := &model.SeriesMetadata{
		TmdbID: 123,
		Title:  "Test Series",
	}

	series := &storage.Series{
		Series: model.Series{
			ID:               1,
			Path:             &path,
			QualityProfileID: 5,
			Monitored:        monitored,
		},
	}

	result := svc.buildTVDetailResult(metadata, &tmdb.SeriesDetailsResponse{}, series, nil)
	assert.Equal(t, string(series.State), result.LibraryStatus)
	assert.Equal(t, path, *result.Path)
	assert.Equal(t, int32(5), *result.QualityProfileID)
	assert.True(t, *result.Monitored)
}
