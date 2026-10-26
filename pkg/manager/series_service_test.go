package manager

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	mockLibrary "github.com/kasuboski/mediaz/pkg/library/mocks"
	"github.com/kasuboski/mediaz/pkg/ptr"
	"github.com/kasuboski/mediaz/pkg/storage"
	storageMocks "github.com/kasuboski/mediaz/pkg/storage/mocks"
	sqlcdb "github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/kasuboski/mediaz/pkg/tmdb"
	tmdbMocks "github.com/kasuboski/mediaz/pkg/tmdb/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type mockMetadataProvider struct {
	metadata *sqlcdb.SeriesMetadatum
	err      error
}

func (m *mockMetadataProvider) GetSeriesMetadata(context.Context, int64) (*sqlcdb.SeriesMetadatum, error) {
	return m.metadata, m.err
}

func newTestSeriesService(ctrl *gomock.Controller) (*SeriesService, *storageMocks.MockStorage, *tmdbMocks.MockITmdb, *mockLibrary.MockLibrary) {
	store := storageMocks.NewMockStorage(ctrl)
	tmdbClient := tmdbMocks.NewMockITmdb(ctrl)
	lib := mockLibrary.NewMockLibrary(ctrl)
	qualityService := NewQualityService(store)
	metadataProvider := &mockMetadataProvider{}
	return NewSeriesService(tmdbClient, lib, store, store, qualityService, metadataProvider), store, tmdbClient, lib
}

func TestSeriesService_UpdateSeriesMonitored(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ctx := context.Background()
	svc, store, _, _ := newTestSeriesService(ctrl)
	want := sqlcdb.GetSeriesRow{Series: sqlcdb.Series{ID: 1, Monitored: 1}, State: string(storage.SeriesStateMissing)}
	store.EXPECT().UpdateSeriesByID(ctx, int64(1), sqlcdb.Series{Monitored: 1}).Return(nil)
	store.EXPECT().GetSeriesByID(ctx, int64(1)).Return(want, nil)
	got, err := svc.UpdateSeriesMonitored(ctx, 1, true)
	require.NoError(t, err)
	assert.Equal(t, &want, got)
}

func TestSeriesService_UpdateSeriesMonitored_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	svc, store, _, _ := newTestSeriesService(ctrl)
	store.EXPECT().UpdateSeriesByID(gomock.Any(), int64(1), gomock.Any()).Return(errors.New("db error"))
	got, err := svc.UpdateSeriesMonitored(context.Background(), 1, true)
	require.Error(t, err)
	assert.Nil(t, got)
}

func TestSeriesService_DeleteSeries(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ctx := context.Background()
	svc, store, _, _ := newTestSeriesService(ctrl)
	series := sqlcdb.GetSeriesRow{Series: sqlcdb.Series{ID: 1, Path: sql.NullString{String: "Test Series", Valid: true}}}
	store.EXPECT().GetSeriesByID(ctx, int64(1)).Return(series, nil)
	store.EXPECT().ListSeasons(ctx, storage.SeasonFilter{SeriesID: ptr.To(int64(1))}).Return([]sqlcdb.ListSeasonsRow{}, nil)
	store.EXPECT().DeleteSeries(ctx, int64(1)).Return(nil)
	require.NoError(t, svc.DeleteSeries(ctx, 1, false))
}

func TestSeriesService_DeleteSeries_WithDirectory(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ctx := context.Background()
	svc, store, _, lib := newTestSeriesService(ctrl)
	series := sqlcdb.GetSeriesRow{Series: sqlcdb.Series{ID: 1, Path: sql.NullString{String: "Test Series", Valid: true}}}
	store.EXPECT().GetSeriesByID(ctx, int64(1)).Return(series, nil)
	store.EXPECT().ListSeasons(ctx, gomock.Any()).Return([]sqlcdb.ListSeasonsRow{}, nil)
	lib.EXPECT().DeleteSeriesDirectory(ctx, "Test Series").Return(nil)
	store.EXPECT().DeleteSeries(ctx, int64(1)).Return(nil)
	require.NoError(t, svc.DeleteSeries(ctx, 1, true))
}

func TestSeriesService_DeleteSeries_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	svc, store, _, _ := newTestSeriesService(ctrl)
	store.EXPECT().GetSeriesByID(gomock.Any(), int64(1)).Return(sqlcdb.GetSeriesRow{}, storage.ErrNotFound)
	err := svc.DeleteSeries(context.Background(), 1, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get series")
}

func TestSeriesService_DeleteSeries_NilPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ctx := context.Background()
	svc, store, _, _ := newTestSeriesService(ctrl)
	store.EXPECT().GetSeriesByID(ctx, int64(1)).Return(sqlcdb.GetSeriesRow{Series: sqlcdb.Series{ID: 1}}, nil)
	store.EXPECT().ListSeasons(ctx, gomock.Any()).Return([]sqlcdb.ListSeasonsRow{}, nil)
	err := svc.DeleteSeries(ctx, 1, true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "series path is nil")
}

func TestSeriesService_DeleteSeries_WithEpisodeFiles_NoDeleteDirectory(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ctx := context.Background()
	svc, store, _, _ := newTestSeriesService(ctrl)
	series := sqlcdb.GetSeriesRow{Series: sqlcdb.Series{ID: 1, Path: sql.NullString{String: "Test Series", Valid: true}}}
	season := sqlcdb.ListSeasonsRow{Season: sqlcdb.Season{ID: 2, SeriesID: 1}}
	store.EXPECT().GetSeriesByID(ctx, int64(1)).Return(series, nil)
	store.EXPECT().ListSeasons(ctx, gomock.Any()).Return([]sqlcdb.ListSeasonsRow{season}, nil)
	store.EXPECT().DeleteSeries(ctx, int64(1)).Return(nil)
	// No episode listing, file deletion, or filesystem deletion is expected.
	require.NoError(t, svc.DeleteSeries(ctx, 1, false))
}

func TestSeriesService_DeleteSeries_WithEpisodeFiles(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ctx := context.Background()
	svc, store, _, lib := newTestSeriesService(ctrl)
	series := sqlcdb.GetSeriesRow{Series: sqlcdb.Series{ID: 1, Path: sql.NullString{String: "Test Series", Valid: true}}}
	season := sqlcdb.ListSeasonsRow{Season: sqlcdb.Season{ID: 2, SeriesID: 1}}
	episode := sqlcdb.ListEpisodesRow{Episode: sqlcdb.Episode{ID: 3, SeasonID: 2, EpisodeFileID: sql.NullInt64{Int64: 10, Valid: true}}}
	store.EXPECT().GetSeriesByID(ctx, int64(1)).Return(series, nil)
	store.EXPECT().ListSeasons(ctx, gomock.Any()).Return([]sqlcdb.ListSeasonsRow{season}, nil)
	store.EXPECT().ListEpisodes(ctx, storage.EpisodeFilter{SeasonID: ptr.To(int64(2))}).Return([]sqlcdb.ListEpisodesRow{episode}, nil)
	lib.EXPECT().DeleteSeriesDirectory(ctx, "Test Series").Return(nil)
	store.EXPECT().DeleteEpisodeFile(ctx, int64(10)).Return(nil)
	store.EXPECT().DeleteSeries(ctx, int64(1)).Return(nil)
	require.NoError(t, svc.DeleteSeries(ctx, 1, true))
}

func TestSeriesService_ListShowsInLibrary(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ctx := context.Background()
	svc, store, _, _ := newTestSeriesService(ctrl)
	store.EXPECT().ListSeries(ctx, storage.SeriesFilter{}).Return([]sqlcdb.ListSeriesRow{{
		Series: sqlcdb.Series{ID: 1, SeriesMetadataID: sql.NullInt64{Int64: 7, Valid: true}, Path: sql.NullString{String: "Test Show", Valid: true}},
		State:  string(storage.SeriesStateMissing),
	}}, nil)
	store.EXPECT().GetSeriesMetadataByID(ctx, int64(7)).Return(sqlcdb.SeriesMetadatum{
		ID: 7, TmdbID: 123, Title: "Test Show", PosterPath: sql.NullString{String: "poster.jpg", Valid: true},
	}, nil)
	shows, err := svc.ListShowsInLibrary(ctx)
	require.NoError(t, err)
	assert.Equal(t, []LibraryShow{{Path: "Test Show", TMDBID: 123, Title: "Test Show", PosterPath: "poster.jpg", State: string(storage.SeriesStateMissing)}}, shows)
}

func TestSeriesService_ListShowsInLibrary_SkipsNoMetadata(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	svc, store, _, _ := newTestSeriesService(ctrl)
	store.EXPECT().ListSeries(gomock.Any(), storage.SeriesFilter{}).Return([]sqlcdb.ListSeriesRow{{Series: sqlcdb.Series{ID: 1}}}, nil)
	shows, err := svc.ListShowsInLibrary(context.Background())
	require.NoError(t, err)
	assert.Empty(t, shows)
}

func TestSeriesService_GetSeriesDetails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ctx := context.Background()
	svc, _, tmdbClient, _ := newTestSeriesService(ctrl)
	tmdbClient.EXPECT().GetSeriesDetails(ctx, 123).Return(&tmdb.SeriesDetails{ID: 123, Name: "Test Series", NumberOfSeasons: 2}, nil)
	got, err := svc.GetSeriesDetails(ctx, 123)
	require.NoError(t, err)
	assert.Equal(t, sqlcdb.SeriesMetadatum{TmdbID: 123, Title: "Test Series", SeasonCount: 2, Overview: sql.NullString{String: "", Valid: true}}, got)
}

func TestSeriesService_SearchTV_EmptyQuery(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	svc, _, _, _ := newTestSeriesService(ctrl)
	_, err := svc.SearchTV(context.Background(), "")
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
	provider := &mockMetadataProvider{}
	svc := NewSeriesService(tmdbClient, lib, store, store, qualityService, provider)
	require.NotNil(t, svc)
	assert.Equal(t, tmdbClient, svc.tmdb)
	assert.Equal(t, lib, svc.library)
	assert.NotNil(t, svc.qualityService)
	assert.Equal(t, provider, svc.metadataProvider)
}

func TestSeriesService_buildTVDetailResult(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	svc, _, _, _ := newTestSeriesService(ctrl)
	metadata := &sqlcdb.SeriesMetadatum{TmdbID: 123, Title: "Test Series", SeasonCount: 3, EpisodeCount: 30, Overview: sql.NullString{String: "A great show", Valid: true}}
	got := svc.buildTVDetailResult(metadata, &tmdb.SeriesDetailsResponse{}, nil, nil)
	assert.Equal(t, &TVDetailResult{TMDBID: 123, Title: "Test Series", Overview: "A great show", SeasonCount: 3, EpisodeCount: 30, LibraryStatus: "Not In Library"}, got)
}

func TestSeriesService_buildTVDetailResult_WithSeries(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	svc, _, _, _ := newTestSeriesService(ctrl)
	path := "Test Show"
	metadata := &sqlcdb.SeriesMetadatum{TmdbID: 123, Title: "Test Series"}
	series := &sqlcdb.GetSeriesRow{Series: sqlcdb.Series{ID: 1, Path: sql.NullString{String: path, Valid: true}, QualityProfileID: 5, Monitored: 1}}
	got := svc.buildTVDetailResult(metadata, &tmdb.SeriesDetailsResponse{}, series, nil)
	assert.Equal(t, &TVDetailResult{TMDBID: 123, Title: "Test Series", LibraryStatus: "", ID: ptr.To(int64(1)), Path: &path, QualityProfileID: ptr.To(int64(5)), Monitored: ptr.To(true), MonitorNewSeasons: ptr.To(false)}, got)
}
