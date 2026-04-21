package manager

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"
	"testing/iotest"
	"time"

	"github.com/kasuboski/mediaz/pkg/ptr"
	storeMocks "github.com/kasuboski/mediaz/pkg/storage/mocks"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/kasuboski/mediaz/pkg/tmdb"
	tmdbMocks "github.com/kasuboski/mediaz/pkg/tmdb/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestMetadataService_GetMovieMetadata(t *testing.T) {
	t.Run("returns existing metadata from storage", func(t *testing.T) {
		ctx := context.Background()
		store := newStore(t, ctx)

		releaseDate, _ := time.Parse(tmdb.ReleaseDateFormat, "2020-06-01")
		year := int32(2020)
		_, err := store.CreateMovieMetadata(ctx, model.MovieMetadata{
			TmdbID:      100,
			Title:       "Stored Movie",
			Runtime:     120,
			ReleaseDate: &releaseDate,
			Year:        &year,
		})
		require.NoError(t, err)

		svc := NewMetadataService(nil, store, nil)
		got, err := svc.GetMovieMetadata(ctx, 100)
		require.NoError(t, err)

		assert.Equal(t, model.MovieMetadata{
			ID:          1,
			TmdbID:      100,
			Title:       "Stored Movie",
			Runtime:     120,
			ReleaseDate: &releaseDate,
			Year:        &year,
		}, *got)
	})

	t.Run("fetches from TMDB when not in storage", func(t *testing.T) {
		ctx := context.Background()
		ctrl := gomock.NewController(t)
		store := newStore(t, ctx)

		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)
		tmdbMock.EXPECT().GetMovieDetails(ctx, 200).Return(&tmdb.MediaDetails{
			ID:       200,
			Title:    ptr.To("TMDB Movie"),
			Runtime:  ptr.To(90),
			Overview: ptr.To("A great film"),
		}, nil)

		svc := NewMetadataService(tmdbMock, store, nil)
		got, err := svc.GetMovieMetadata(ctx, 200)
		require.NoError(t, err)

		assert.Equal(t, model.MovieMetadata{
			ID:       1,
			TmdbID:   200,
			Title:    "TMDB Movie",
			Runtime:  90,
			Overview: ptr.To("A great film"),
		}, *got)
	})

	t.Run("returns error from storage on non-not-found error", func(t *testing.T) {
		ctx := context.Background()
		ctrl := gomock.NewController(t)

		store := storeMocks.NewMockStorage(ctrl)
		store.EXPECT().GetMovieMetadata(ctx, gomock.Any()).Return(nil, errors.New("db error"))

		svc := NewMetadataService(nil, store, nil)
		_, err := svc.GetMovieMetadata(ctx, 1)
		require.Error(t, err)
		assert.Equal(t, "db error", err.Error())
	})

	t.Run("returns error when TMDB fetch fails", func(t *testing.T) {
		ctx := context.Background()
		ctrl := gomock.NewController(t)
		store := newStore(t, ctx)

		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)
		tmdbMock.EXPECT().GetMovieDetails(ctx, 300).Return(nil, errors.New("tmdb unavailable"))

		svc := NewMetadataService(tmdbMock, store, nil)
		_, err := svc.GetMovieMetadata(ctx, 300)
		require.Error(t, err)
		assert.Equal(t, "tmdb unavailable", err.Error())
	})
}

func TestMetadataService_GetMovieMetadataByID(t *testing.T) {
	t.Run("returns metadata by storage ID", func(t *testing.T) {
		ctx := context.Background()
		store := newStore(t, ctx)

		id, err := store.CreateMovieMetadata(ctx, model.MovieMetadata{
			TmdbID:  42,
			Title:   "By ID Movie",
			Runtime: 95,
		})
		require.NoError(t, err)

		svc := NewMetadataService(nil, store, nil)
		got, err := svc.GetMovieMetadataByID(ctx, int32(id))
		require.NoError(t, err)

		assert.Equal(t, model.MovieMetadata{
			ID:      int32(id),
			TmdbID:  42,
			Title:   "By ID Movie",
			Runtime: 95,
		}, *got)
	})

	t.Run("returns error when not found", func(t *testing.T) {
		ctx := context.Background()
		store := newStore(t, ctx)

		svc := NewMetadataService(nil, store, nil)
		_, err := svc.GetMovieMetadataByID(ctx, 9999)
		require.Error(t, err)
	})
}

func TestMetadataService_UpdateMovieMetadataFromTMDB(t *testing.T) {
	t.Run("updates existing metadata with TMDB data", func(t *testing.T) {
		ctx := context.Background()
		ctrl := gomock.NewController(t)
		store := newStore(t, ctx)

		id, err := store.CreateMovieMetadata(ctx, model.MovieMetadata{
			TmdbID:  50,
			Title:   "Old Title",
			Runtime: 80,
		})
		require.NoError(t, err)

		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)
		tmdbMock.EXPECT().GetMovieDetails(ctx, 50).Return(&tmdb.MediaDetails{
			ID:       50,
			Title:    ptr.To("Updated Title"),
			Runtime:  ptr.To(85),
			Overview: ptr.To("New overview"),
		}, nil)

		svc := NewMetadataService(tmdbMock, store, nil)
		got, err := svc.UpdateMovieMetadataFromTMDB(ctx, 50)
		require.NoError(t, err)

		assert.Equal(t, model.MovieMetadata{
			ID:       int32(id),
			TmdbID:   50,
			Title:    "Updated Title",
			Runtime:  85,
			Overview: ptr.To("New overview"),
		}, *got)

		fromStore, err := svc.GetMovieMetadataByID(ctx, int32(id))
		require.NoError(t, err)
		assert.Equal(t, model.MovieMetadata{
			ID:       int32(id),
			TmdbID:   50,
			Title:    "Updated Title",
			Runtime:  85,
			Overview: ptr.To("New overview"),
		}, *fromStore)
	})

	t.Run("returns error when TMDB call fails", func(t *testing.T) {
		ctx := context.Background()
		ctrl := gomock.NewController(t)

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)
		tmdbMock.EXPECT().GetMovieDetails(ctx, 51).Return(nil, errors.New("tmdb error"))

		svc := NewMetadataService(tmdbMock, store, nil)
		_, err := svc.UpdateMovieMetadataFromTMDB(ctx, 51)
		require.Error(t, err)
		assert.Equal(t, "tmdb error", err.Error())
	})

	t.Run("returns error when existing metadata not found", func(t *testing.T) {
		ctx := context.Background()
		ctrl := gomock.NewController(t)
		store := newStore(t, ctx)

		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)
		tmdbMock.EXPECT().GetMovieDetails(ctx, 52).Return(&tmdb.MediaDetails{
			ID:      52,
			Title:   ptr.To("Movie"),
			Runtime: ptr.To(100),
		}, nil)

		svc := NewMetadataService(tmdbMock, store, nil)
		_, err := svc.UpdateMovieMetadataFromTMDB(ctx, 52)
		require.Error(t, err)
	})
}

func TestMetadataService_GetSeriesMetadata(t *testing.T) {
	t.Run("returns existing series metadata from storage", func(t *testing.T) {
		ctx := context.Background()
		store := newStore(t, ctx)

		airDate, _ := time.Parse(tmdb.ReleaseDateFormat, "2021-03-15")
		_, err := store.CreateSeriesMetadata(ctx, model.SeriesMetadata{
			TmdbID:       500,
			Title:        "Stored Series",
			SeasonCount:  3,
			EpisodeCount: 30,
			FirstAirDate: &airDate,
			Status:       "Ended",
		})
		require.NoError(t, err)

		svc := NewMetadataService(nil, nil, store)
		got, err := svc.GetSeriesMetadata(ctx, 500)
		require.NoError(t, err)

		assert.Equal(t, model.SeriesMetadata{
			ID:           1,
			TmdbID:       500,
			Title:        "Stored Series",
			SeasonCount:  3,
			EpisodeCount: 30,
			FirstAirDate: &airDate,
			Status:       "Ended",
		}, *got)
	})

	t.Run("loads from TMDB with full hierarchy when not found", func(t *testing.T) {
		ctx := context.Background()
		ctrl := gomock.NewController(t)
		store := newStore(t, ctx)

		airDate, _ := time.Parse(tmdb.ReleaseDateFormat, "2022-01-10")
		epRuntime := int32(44)

		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)
		tmdbMock.EXPECT().GetSeriesDetails(ctx, 501).Return(&tmdb.SeriesDetails{
			ID:              501,
			Name:            "New Series",
			FirstAirDate:    "2022-01-10",
			NumberOfSeasons: 1,
			Status:          "Continuing",
			Seasons: []tmdb.Season{
				{
					ID:           10,
					Name:         "Season 1",
					AirDate:      "2022-01-10",
					SeasonNumber: 1,
					Episodes: []tmdb.Episode{
						{
							ID:            100,
							Name:          "Pilot",
							AirDate:       "2022-01-10",
							EpisodeNumber: 1,
							Runtime:       44,
							Overview:      "Episode overview",
						},
					},
				},
			},
		}, nil)
		tmdbMock.EXPECT().TvSeriesExternalIds(ctx, int32(501)).Return(
			&http.Response{StatusCode: 404, Body: io.NopCloser(bytes.NewBufferString(""))}, nil,
		)
		tmdbMock.EXPECT().TvSeriesWatchProviders(ctx, int32(501)).Return(
			&http.Response{StatusCode: 404, Body: io.NopCloser(bytes.NewBufferString(""))}, nil,
		)

		svc := NewMetadataService(tmdbMock, nil, store)
		series, err := svc.GetSeriesMetadata(ctx, 501)
		require.NoError(t, err)

		assert.Equal(t, model.SeriesMetadata{
			ID:           1,
			TmdbID:       501,
			Title:        "New Series",
			SeasonCount:  1,
			FirstAirDate: &airDate,
			Status:       "Continuing",
			Overview:     ptr.To(""),
		}, *series)

		seasons, err := store.ListSeasonMetadata(ctx)
		require.NoError(t, err)
		assert.Equal(t, []*model.SeasonMetadata{
			{ID: 1, SeriesMetadataID: 1, TmdbID: 10, Title: "Season 1", Number: 1, AirDate: &airDate, Overview: ptr.To("")},
		}, seasons)

		episodes, err := store.ListEpisodeMetadata(ctx)
		require.NoError(t, err)
		assert.Equal(t, []*model.EpisodeMetadata{
			{ID: 1, SeasonMetadataID: 1, TmdbID: 100, Title: "Pilot", Number: 1, AirDate: &airDate, Runtime: &epRuntime, Overview: ptr.To("Episode overview")},
		}, episodes)
	})

	t.Run("returns error on storage failure", func(t *testing.T) {
		ctx := context.Background()
		ctrl := gomock.NewController(t)

		store := storeMocks.NewMockStorage(ctrl)
		store.EXPECT().GetSeriesMetadata(ctx, gomock.Any()).Return(nil, errors.New("storage down"))

		svc := NewMetadataService(nil, nil, store)
		_, err := svc.GetSeriesMetadata(ctx, 502)
		require.Error(t, err)
		assert.Equal(t, "storage down", err.Error())
	})
}

func TestMetadataService_UpdateSeriesMetadataFromTMDB(t *testing.T) {
	t.Run("updates existing series metadata", func(t *testing.T) {
		ctx := context.Background()
		ctrl := gomock.NewController(t)
		store := newStore(t, ctx)

		airDate, _ := time.Parse(tmdb.ReleaseDateFormat, "2019-04-14")
		_, err := store.CreateSeriesMetadata(ctx, model.SeriesMetadata{
			TmdbID:       600,
			Title:        "Old Title",
			SeasonCount:  7,
			EpisodeCount: 73,
			FirstAirDate: &airDate,
			Status:       "",
		})
		require.NoError(t, err)

		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)
		tmdbMock.EXPECT().GetSeriesDetails(ctx, 600).Return(&tmdb.SeriesDetails{
			ID:              600,
			Name:            "Updated Series",
			FirstAirDate:    "2019-04-14",
			NumberOfSeasons: 8,
			Status:          "Ended",
		}, nil)
		tmdbMock.EXPECT().TvSeriesExternalIds(ctx, int32(600)).Return(
			&http.Response{StatusCode: 404, Body: io.NopCloser(bytes.NewBufferString(""))}, nil,
		)
		tmdbMock.EXPECT().TvSeriesWatchProviders(ctx, int32(600)).Return(
			&http.Response{StatusCode: 404, Body: io.NopCloser(bytes.NewBufferString(""))}, nil,
		)

		svc := NewMetadataService(tmdbMock, nil, store)
		got, err := svc.UpdateSeriesMetadataFromTMDB(ctx, 600)
		require.NoError(t, err)

		assert.Equal(t, model.SeriesMetadata{
			ID:           1,
			TmdbID:       600,
			Title:        "Updated Series",
			SeasonCount:  8,
			FirstAirDate: &airDate,
			Status:       "Ended",
			Overview:     ptr.To(""),
		}, *got)
	})

	t.Run("returns error when TMDB call fails", func(t *testing.T) {
		ctx := context.Background()
		ctrl := gomock.NewController(t)

		store := storeMocks.NewMockStorage(ctrl)
		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)
		tmdbMock.EXPECT().GetSeriesDetails(ctx, 601).Return(nil, errors.New("tmdb down"))

		svc := NewMetadataService(tmdbMock, nil, store)
		_, err := svc.UpdateSeriesMetadataFromTMDB(ctx, 601)
		require.Error(t, err)
		assert.Equal(t, "tmdb down", err.Error())
	})

	t.Run("returns error when series not in storage", func(t *testing.T) {
		ctx := context.Background()
		ctrl := gomock.NewController(t)
		store := newStore(t, ctx)

		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)
		tmdbMock.EXPECT().GetSeriesDetails(ctx, 602).Return(&tmdb.SeriesDetails{
			ID:           602,
			Name:         "Series",
			FirstAirDate: "2020-01-01",
		}, nil)

		svc := NewMetadataService(tmdbMock, nil, store)
		_, err := svc.UpdateSeriesMetadataFromTMDB(ctx, 602)
		require.Error(t, err)
	})
}

func TestMetadataService_RefreshMovieMetadata(t *testing.T) {
	t.Run("refreshes all movies when no IDs provided", func(t *testing.T) {
		ctx := context.Background()
		ctrl := gomock.NewController(t)
		store := newStore(t, ctx)

		_, err := store.CreateMovieMetadata(ctx, model.MovieMetadata{TmdbID: 10, Title: "Movie A", Runtime: 90})
		require.NoError(t, err)
		_, err = store.CreateMovieMetadata(ctx, model.MovieMetadata{TmdbID: 11, Title: "Movie B", Runtime: 100})
		require.NoError(t, err)

		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)
		tmdbMock.EXPECT().GetMovieDetails(ctx, 10).Return(&tmdb.MediaDetails{
			ID: 10, Title: ptr.To("Movie A Updated"), Runtime: ptr.To(91),
		}, nil)
		tmdbMock.EXPECT().GetMovieDetails(ctx, 11).Return(&tmdb.MediaDetails{
			ID: 11, Title: ptr.To("Movie B Updated"), Runtime: ptr.To(101),
		}, nil)

		svc := NewMetadataService(tmdbMock, store, nil)
		err = svc.RefreshMovieMetadata(ctx)
		require.NoError(t, err)

		a, err := svc.GetMovieMetadata(ctx, 10)
		require.NoError(t, err)
		assert.Equal(t, model.MovieMetadata{ID: 1, TmdbID: 10, Title: "Movie A Updated", Runtime: 91}, *a)

		b, err := svc.GetMovieMetadata(ctx, 11)
		require.NoError(t, err)
		assert.Equal(t, model.MovieMetadata{ID: 2, TmdbID: 11, Title: "Movie B Updated", Runtime: 101}, *b)
	})

	t.Run("refreshes specific movie IDs", func(t *testing.T) {
		ctx := context.Background()
		ctrl := gomock.NewController(t)
		store := newStore(t, ctx)

		_, err := store.CreateMovieMetadata(ctx, model.MovieMetadata{TmdbID: 20, Title: "Movie C", Runtime: 110})
		require.NoError(t, err)

		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)
		tmdbMock.EXPECT().GetMovieDetails(ctx, 20).Return(&tmdb.MediaDetails{
			ID: 20, Title: ptr.To("Movie C Refreshed"), Runtime: ptr.To(112),
		}, nil)

		svc := NewMetadataService(tmdbMock, store, nil)
		err = svc.RefreshMovieMetadata(ctx, 20)
		require.NoError(t, err)

		got, err := svc.GetMovieMetadata(ctx, 20)
		require.NoError(t, err)
		assert.Equal(t, model.MovieMetadata{ID: 1, TmdbID: 20, Title: "Movie C Refreshed", Runtime: 112}, *got)
	})

	t.Run("returns aggregated error when UpdateMovieMetadataFromTMDB fails for explicit IDs", func(t *testing.T) {
		ctx := context.Background()
		ctrl := gomock.NewController(t)

		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)
		tmdbErr := errors.New("tmdb unavailable")
		tmdbMock.EXPECT().GetMovieDetails(ctx, 30).Return(nil, tmdbErr)
		tmdbMock.EXPECT().GetMovieDetails(ctx, 31).Return(nil, tmdbErr)

		svc := NewMetadataService(tmdbMock, nil, nil)
		err := svc.RefreshMovieMetadata(ctx, 30, 31)
		require.Error(t, err)
		assert.ErrorContains(t, err, "tmdb unavailable")
	})

	t.Run("returns aggregated error when UpdateMovieMetadataFromTMDB fails during list", func(t *testing.T) {
		ctx := context.Background()
		ctrl := gomock.NewController(t)
		store := newStore(t, ctx)

		_, err := store.CreateMovieMetadata(ctx, model.MovieMetadata{TmdbID: 40, Title: "Failing Movie", Runtime: 90})
		require.NoError(t, err)

		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)
		tmdbErr := errors.New("tmdb error")
		tmdbMock.EXPECT().GetMovieDetails(ctx, 40).Return(nil, tmdbErr)

		svc := NewMetadataService(tmdbMock, store, nil)
		err = svc.RefreshMovieMetadata(ctx)
		require.Error(t, err)
		assert.ErrorContains(t, err, "tmdb error")
	})

	t.Run("returns context error immediately on cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		ctrl := gomock.NewController(t)

		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)
		svc := NewMetadataService(tmdbMock, nil, nil)
		err := svc.RefreshMovieMetadata(ctx, 999)
		require.ErrorIs(t, err, context.Canceled)
	})
}

func TestMetadataService_RefreshSeriesMetadata(t *testing.T) {
	t.Run("refreshes series with empty status when no IDs provided", func(t *testing.T) {
		ctx := context.Background()
		ctrl := gomock.NewController(t)
		store := newStore(t, ctx)

		airDate, _ := time.Parse(tmdb.ReleaseDateFormat, "2020-01-01")
		_, err := store.CreateSeriesMetadata(ctx, model.SeriesMetadata{
			TmdbID:       700,
			Title:        "Ongoing Show",
			FirstAirDate: &airDate,
			Status:       "",
		})
		require.NoError(t, err)

		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)
		tmdbMock.EXPECT().GetSeriesDetails(ctx, 700).Return(&tmdb.SeriesDetails{
			ID:           700,
			Name:         "Ongoing Show Refreshed",
			FirstAirDate: "2020-01-01",
			Status:       "Continuing",
		}, nil)
		tmdbMock.EXPECT().TvSeriesExternalIds(ctx, int32(700)).Return(
			&http.Response{StatusCode: 404, Body: io.NopCloser(bytes.NewBufferString(""))}, nil,
		)
		tmdbMock.EXPECT().TvSeriesWatchProviders(ctx, int32(700)).Return(
			&http.Response{StatusCode: 404, Body: io.NopCloser(bytes.NewBufferString(""))}, nil,
		)

		svc := NewMetadataService(tmdbMock, nil, store)
		err = svc.RefreshSeriesMetadata(ctx)
		require.NoError(t, err)

		got, err := svc.GetSeriesMetadata(ctx, 700)
		require.NoError(t, err)
		got.LastInfoSync = nil
		assert.Equal(t, model.SeriesMetadata{
			ID:           1,
			TmdbID:       700,
			Title:        "Ongoing Show Refreshed",
			FirstAirDate: &airDate,
			Status:       "Continuing",
			Overview:     ptr.To(""),
		}, *got)
	})

	t.Run("refreshes specific series IDs", func(t *testing.T) {
		ctx := context.Background()
		ctrl := gomock.NewController(t)
		store := newStore(t, ctx)

		airDate, _ := time.Parse(tmdb.ReleaseDateFormat, "2018-06-01")
		_, err := store.CreateSeriesMetadata(ctx, model.SeriesMetadata{
			TmdbID:       800,
			Title:        "Show To Refresh",
			FirstAirDate: &airDate,
			Status:       "Ended",
		})
		require.NoError(t, err)

		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)
		tmdbMock.EXPECT().GetSeriesDetails(ctx, 800).Return(&tmdb.SeriesDetails{
			ID:           800,
			Name:         "Show Refreshed",
			FirstAirDate: "2018-06-01",
			Status:       "Ended",
		}, nil)
		tmdbMock.EXPECT().TvSeriesExternalIds(ctx, int32(800)).Return(
			&http.Response{StatusCode: 404, Body: io.NopCloser(bytes.NewBufferString(""))}, nil,
		)
		tmdbMock.EXPECT().TvSeriesWatchProviders(ctx, int32(800)).Return(
			&http.Response{StatusCode: 404, Body: io.NopCloser(bytes.NewBufferString(""))}, nil,
		)

		svc := NewMetadataService(tmdbMock, nil, store)
		err = svc.RefreshSeriesMetadata(ctx, 800)
		require.NoError(t, err)

		got, err := svc.GetSeriesMetadata(ctx, 800)
		require.NoError(t, err)
		got.LastInfoSync = nil
		assert.Equal(t, model.SeriesMetadata{
			ID:           1,
			TmdbID:       800,
			Title:        "Show Refreshed",
			FirstAirDate: &airDate,
			Status:       "Ended",
			Overview:     ptr.To(""),
		}, *got)
	})

	t.Run("returns aggregated error when UpdateSeriesMetadataFromTMDB fails for explicit IDs", func(t *testing.T) {
		ctx := context.Background()
		ctrl := gomock.NewController(t)

		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)
		tmdbErr := errors.New("tmdb unavailable")
		tmdbMock.EXPECT().GetSeriesDetails(ctx, 901).Return(nil, tmdbErr)
		tmdbMock.EXPECT().GetSeriesDetails(ctx, 902).Return(nil, tmdbErr)

		svc := NewMetadataService(tmdbMock, nil, nil)
		err := svc.RefreshSeriesMetadata(ctx, 901, 902)
		require.Error(t, err)
		assert.ErrorContains(t, err, "tmdb unavailable")
	})

	t.Run("returns aggregated error when UpdateSeriesMetadataFromTMDB fails during list", func(t *testing.T) {
		ctx := context.Background()
		ctrl := gomock.NewController(t)
		store := newStore(t, ctx)

		airDate, _ := time.Parse(tmdb.ReleaseDateFormat, "2021-03-01")
		_, err := store.CreateSeriesMetadata(ctx, model.SeriesMetadata{
			TmdbID:       903,
			Title:        "Failing Show",
			FirstAirDate: &airDate,
			Status:       "",
		})
		require.NoError(t, err)

		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)
		tmdbErr := errors.New("tmdb error")
		tmdbMock.EXPECT().GetSeriesDetails(ctx, 903).Return(nil, tmdbErr)

		svc := NewMetadataService(tmdbMock, nil, store)
		err = svc.RefreshSeriesMetadata(ctx)
		require.Error(t, err)
		assert.ErrorContains(t, err, "tmdb error")
	})

	t.Run("returns context error immediately on cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		ctrl := gomock.NewController(t)

		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)
		svc := NewMetadataService(tmdbMock, nil, nil)
		err := svc.RefreshSeriesMetadata(ctx, 999)
		require.ErrorIs(t, err, context.Canceled)
	})
}

func TestMetadataService_fetchExternalIDs(t *testing.T) {
	t.Run("serializes external IDs from TMDB response", func(t *testing.T) {
		ctx := context.Background()
		ctrl := gomock.NewController(t)

		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)
		tmdbMock.EXPECT().TvSeriesExternalIds(ctx, int32(1)).Return(
			&http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewBufferString(`{"imdb_id":"tt0000001","tvdb_id":12345}`))},
			nil,
		)

		svc := NewMetadataService(tmdbMock, nil, nil)
		got, err := svc.fetchExternalIDs(ctx, 1)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, `{"imdb_id":"tt0000001","tvdb_id":12345}`, *got)
	})

	t.Run("returns nil without error on API failure", func(t *testing.T) {
		ctx := context.Background()
		ctrl := gomock.NewController(t)

		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)
		tmdbMock.EXPECT().TvSeriesExternalIds(ctx, int32(2)).Return(nil, errors.New("api error"))

		svc := NewMetadataService(tmdbMock, nil, nil)
		got, err := svc.fetchExternalIDs(ctx, 2)
		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("returns nil without error on non-200 status", func(t *testing.T) {
		ctx := context.Background()
		ctrl := gomock.NewController(t)

		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)
		tmdbMock.EXPECT().TvSeriesExternalIds(ctx, int32(3)).Return(
			&http.Response{StatusCode: 404, Body: io.NopCloser(bytes.NewBufferString(""))}, nil,
		)

		svc := NewMetadataService(tmdbMock, nil, nil)
		got, err := svc.fetchExternalIDs(ctx, 3)
		require.NoError(t, err)
		assert.Nil(t, got)
	})
}

func TestMetadataService_fetchWatchProviders(t *testing.T) {
	t.Run("serializes US watch providers from TMDB response", func(t *testing.T) {
		ctx := context.Background()
		ctrl := gomock.NewController(t)

		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)
		tmdbMock.EXPECT().TvSeriesWatchProviders(ctx, int32(1)).Return(
			&http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString(`{"results":{"US":{"flatrate":[{"provider_id":8,"provider_name":"Netflix","logo_path":"/n.png"}]}}}`)),
			},
			nil,
		)

		svc := NewMetadataService(tmdbMock, nil, nil)
		got, err := svc.fetchWatchProviders(ctx, 1)
		require.NoError(t, err)
		require.NotNil(t, got)

		data, err := DeserializeWatchProviders(got)
		require.NoError(t, err)
		assert.Equal(t, &WatchProvidersData{
			US: WatchProviderRegionData{
				Flatrate: []WatchProviderData{
					{ProviderID: 8, Name: "Netflix", LogoPath: ptr.To("/n.png")},
				},
			},
		}, data)
	})

	t.Run("returns nil without error on API failure", func(t *testing.T) {
		ctx := context.Background()
		ctrl := gomock.NewController(t)

		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)
		tmdbMock.EXPECT().TvSeriesWatchProviders(ctx, int32(2)).Return(nil, errors.New("api error"))

		svc := NewMetadataService(tmdbMock, nil, nil)
		got, err := svc.fetchWatchProviders(ctx, 2)
		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("returns nil without error on non-200 status", func(t *testing.T) {
		ctx := context.Background()
		ctrl := gomock.NewController(t)

		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)
		tmdbMock.EXPECT().TvSeriesWatchProviders(ctx, int32(3)).Return(
			&http.Response{StatusCode: 500, Body: io.NopCloser(bytes.NewBufferString(""))}, nil,
		)

		svc := NewMetadataService(tmdbMock, nil, nil)
		got, err := svc.fetchWatchProviders(ctx, 3)
		require.NoError(t, err)
		assert.Nil(t, got)
	})
}

func TestMetadataService_RefreshSeriesMetadataFromTMDB(t *testing.T) {
	t.Run("creates full series hierarchy", func(t *testing.T) {
		ctx := context.Background()
		ctrl := gomock.NewController(t)
		store := newStore(t, ctx)

		airDate, _ := time.Parse(tmdb.ReleaseDateFormat, "2015-04-12")
		epRuntime := int32(60)

		tmdbMock := tmdbMocks.NewMockITmdb(ctrl)
		tmdbMock.EXPECT().GetSeriesDetails(ctx, 900).Return(&tmdb.SeriesDetails{
			ID:              900,
			Name:            "Fresh Series",
			FirstAirDate:    "2015-04-12",
			NumberOfSeasons: 1,
			Status:          "Ended",
			Seasons: []tmdb.Season{
				{
					ID:           20,
					Name:         "Season 1",
					AirDate:      "2015-04-12",
					SeasonNumber: 1,
					Episodes: []tmdb.Episode{
						{ID: 200, Name: "Episode One", AirDate: "2015-04-12", EpisodeNumber: 1, Runtime: 60},
					},
				},
			},
		}, nil)
		tmdbMock.EXPECT().TvSeriesExternalIds(ctx, int32(900)).Return(
			&http.Response{StatusCode: 404, Body: io.NopCloser(bytes.NewBufferString(""))}, nil,
		)
		tmdbMock.EXPECT().TvSeriesWatchProviders(ctx, int32(900)).Return(
			&http.Response{StatusCode: 404, Body: io.NopCloser(bytes.NewBufferString(""))}, nil,
		)

		svc := NewMetadataService(tmdbMock, nil, store)
		series, err := svc.RefreshSeriesMetadataFromTMDB(ctx, 900)
		require.NoError(t, err)

		assert.Equal(t, model.SeriesMetadata{
			ID:           1,
			TmdbID:       900,
			Title:        "Fresh Series",
			SeasonCount:  1,
			FirstAirDate: &airDate,
			Status:       "Ended",
			Overview:     ptr.To(""),
		}, *series)

		seasons, err := store.ListSeasonMetadata(ctx)
		require.NoError(t, err)
		assert.Equal(t, []*model.SeasonMetadata{
			{ID: 1, SeriesMetadataID: 1, TmdbID: 20, Title: "Season 1", Number: 1, AirDate: &airDate, Overview: ptr.To("")},
		}, seasons)

		episodes, err := store.ListEpisodeMetadata(ctx)
		require.NoError(t, err)
		assert.Equal(t, []*model.EpisodeMetadata{
			{ID: 1, SeasonMetadataID: 1, TmdbID: 200, Title: "Episode One", Number: 1, AirDate: &airDate, Runtime: &epRuntime, Overview: ptr.To("")},
		}, episodes)
	})
}

func TestFromMediaDetails(t *testing.T) {
	t.Run("maps basic fields", func(t *testing.T) {
		got := FromMediaDetails(tmdb.MediaDetails{
			ID:      42,
			Title:   ptr.To("Test Movie"),
			Runtime: ptr.To(110),
		})
		assert.Equal(t, model.MovieMetadata{TmdbID: 42, Title: "Test Movie", Runtime: 110}, got)
	})

	t.Run("maps poster path to images", func(t *testing.T) {
		got := FromMediaDetails(tmdb.MediaDetails{
			ID:         1,
			Title:      ptr.To("Movie"),
			Runtime:    ptr.To(90),
			PosterPath: ptr.To("/poster.jpg"),
		})
		assert.Equal(t, model.MovieMetadata{TmdbID: 1, Title: "Movie", Runtime: 90, Images: "/poster.jpg"}, got)
	})

	t.Run("maps genres as comma-separated string", func(t *testing.T) {
		genres := []tmdb.Genre{{Name: "Action"}, {Name: "Drama"}}
		got := FromMediaDetails(tmdb.MediaDetails{
			ID:      1,
			Title:   ptr.To("Movie"),
			Runtime: ptr.To(100),
			Genres:  &genres,
		})
		assert.Equal(t, model.MovieMetadata{TmdbID: 1, Title: "Movie", Runtime: 100, Genres: ptr.To("Action,Drama")}, got)
	})

	t.Run("maps first production company to studio", func(t *testing.T) {
		companies := []tmdb.ProductionCompany{{Name: ptr.To("Studio A")}, {Name: ptr.To("Studio B")}}
		got := FromMediaDetails(tmdb.MediaDetails{
			ID:                  1,
			Title:               ptr.To("Movie"),
			Runtime:             ptr.To(100),
			ProductionCompanies: &companies,
		})
		assert.Equal(t, model.MovieMetadata{TmdbID: 1, Title: "Movie", Runtime: 100, Studio: ptr.To("Studio A")}, got)
	})

	t.Run("maps collection info", func(t *testing.T) {
		coll := any(map[string]any{"id": float64(99), "name": "The Collection"})
		got := FromMediaDetails(tmdb.MediaDetails{
			ID:                  1,
			Title:               ptr.To("Movie"),
			Runtime:             ptr.To(100),
			BelongsToCollection: &coll,
		})
		assert.Equal(t, model.MovieMetadata{
			TmdbID:           1,
			Title:            "Movie",
			Runtime:          100,
			CollectionTmdbID: ptr.To(int32(99)),
			CollectionTitle:  ptr.To("The Collection"),
		}, got)
	})

	t.Run("maps release date and year", func(t *testing.T) {
		releaseDate, _ := time.Parse(tmdb.ReleaseDateFormat, "2021-07-15")
		year := int32(2021)
		got := FromMediaDetails(tmdb.MediaDetails{
			ID:          1,
			Title:       ptr.To("Movie"),
			Runtime:     ptr.To(100),
			ReleaseDate: ptr.To("2021-07-15"),
		})
		assert.Equal(t, model.MovieMetadata{TmdbID: 1, Title: "Movie", Runtime: 100, ReleaseDate: &releaseDate, Year: &year}, got)
	})

	t.Run("ignores invalid release date", func(t *testing.T) {
		got := FromMediaDetails(tmdb.MediaDetails{
			ID:          1,
			Title:       ptr.To("Movie"),
			Runtime:     ptr.To(100),
			ReleaseDate: ptr.To("not-a-date"),
		})
		assert.Equal(t, model.MovieMetadata{TmdbID: 1, Title: "Movie", Runtime: 100}, got)
	})
}

func TestFromSeriesDetails(t *testing.T) {
	t.Run("maps basic fields", func(t *testing.T) {
		airDate, _ := time.Parse(tmdb.ReleaseDateFormat, "2020-03-01")
		got, err := FromSeriesDetails(tmdb.SeriesDetails{
			ID:               10,
			Name:             "My Show",
			FirstAirDate:     "2020-03-01",
			NumberOfSeasons:  3,
			NumberOfEpisodes: 30,
			Status:           "Continuing",
			Overview:         "A cool show",
		})
		require.NoError(t, err)
		assert.Equal(t, model.SeriesMetadata{
			TmdbID:       10,
			Title:        "My Show",
			SeasonCount:  3,
			EpisodeCount: 30,
			Status:       "Continuing",
			FirstAirDate: &airDate,
			Overview:     ptr.To("A cool show"),
		}, got)
	})

	t.Run("maps poster path when non-empty", func(t *testing.T) {
		airDate, _ := time.Parse(tmdb.ReleaseDateFormat, "2021-01-01")
		got, err := FromSeriesDetails(tmdb.SeriesDetails{
			ID:           1,
			Name:         "Show",
			FirstAirDate: "2021-01-01",
			PosterPath:   "/poster.jpg",
		})
		require.NoError(t, err)
		assert.Equal(t, model.SeriesMetadata{TmdbID: 1, Title: "Show", FirstAirDate: &airDate, Overview: ptr.To(""), PosterPath: ptr.To("/poster.jpg")}, got)
	})

	t.Run("nil poster and air date when empty", func(t *testing.T) {
		got, err := FromSeriesDetails(tmdb.SeriesDetails{ID: 1, Name: "Show"})
		require.NoError(t, err)
		assert.Equal(t, model.SeriesMetadata{TmdbID: 1, Title: "Show", Overview: ptr.To("")}, got)
	})

	t.Run("returns error on invalid first air date", func(t *testing.T) {
		_, err := FromSeriesDetails(tmdb.SeriesDetails{ID: 1, Name: "Show", FirstAirDate: "bad-date"})
		require.Error(t, err)
	})
}

func TestFromSeriesSeasons(t *testing.T) {
	t.Run("maps basic season fields", func(t *testing.T) {
		airDate, _ := time.Parse(tmdb.ReleaseDateFormat, "2022-05-10")
		got := FromSeriesSeasons(tmdb.Season{
			ID:           55,
			Name:         "Season 2",
			SeasonNumber: 2,
			AirDate:      "2022-05-10",
			Overview:     "Second season",
		})
		assert.Equal(t, model.SeasonMetadata{TmdbID: 55, Title: "Season 2", Number: 2, AirDate: &airDate, Overview: ptr.To("Second season")}, got)
	})

	t.Run("nil air date when empty", func(t *testing.T) {
		got := FromSeriesSeasons(tmdb.Season{ID: 1, Name: "Season 1", SeasonNumber: 1})
		assert.Equal(t, model.SeasonMetadata{TmdbID: 1, Title: "Season 1", Number: 1, Overview: ptr.To("")}, got)
	})

	t.Run("nil air date on invalid date", func(t *testing.T) {
		got := FromSeriesSeasons(tmdb.Season{ID: 1, Name: "Season 1", SeasonNumber: 1, AirDate: "not-a-date"})
		assert.Equal(t, model.SeasonMetadata{TmdbID: 1, Title: "Season 1", Number: 1, Overview: ptr.To("")}, got)
	})
}

func TestFromSeriesEpisodes(t *testing.T) {
	t.Run("maps basic episode fields", func(t *testing.T) {
		airDate, _ := time.Parse(tmdb.ReleaseDateFormat, "2022-05-17")
		runtime := int32(45)
		got := FromSeriesEpisodes(tmdb.Episode{
			ID:            77,
			Name:          "The Pilot",
			EpisodeNumber: 1,
			AirDate:       "2022-05-17",
			Runtime:       45,
			Overview:      "First episode",
		})
		assert.Equal(t, model.EpisodeMetadata{TmdbID: 77, Title: "The Pilot", Number: 1, AirDate: &airDate, Runtime: &runtime, Overview: ptr.To("First episode")}, got)
	})

	t.Run("maps still path when non-empty", func(t *testing.T) {
		runtime := int32(0)
		got := FromSeriesEpisodes(tmdb.Episode{ID: 1, Name: "Ep", EpisodeNumber: 1, StillPath: "/still.jpg"})
		assert.Equal(t, model.EpisodeMetadata{TmdbID: 1, Title: "Ep", Number: 1, Runtime: &runtime, Overview: ptr.To(""), StillPath: ptr.To("/still.jpg")}, got)
	})

	t.Run("nil still path and air date when empty", func(t *testing.T) {
		runtime := int32(0)
		got := FromSeriesEpisodes(tmdb.Episode{ID: 1, Name: "Ep", EpisodeNumber: 1})
		assert.Equal(t, model.EpisodeMetadata{TmdbID: 1, Title: "Ep", Number: 1, Runtime: &runtime, Overview: ptr.To("")}, got)
	})

	t.Run("nil air date on invalid date", func(t *testing.T) {
		runtime := int32(30)
		got := FromSeriesEpisodes(tmdb.Episode{ID: 1, Name: "Ep", EpisodeNumber: 1, AirDate: "bad", Runtime: 30})
		assert.Equal(t, model.EpisodeMetadata{TmdbID: 1, Title: "Ep", Number: 1, Runtime: &runtime, Overview: ptr.To("")}, got)
	})
}

func TestParseTMDBDate(t *testing.T) {
	t.Run("returns nil for empty string", func(t *testing.T) {
		got, err := parseTMDBDate("")
		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("parses valid date", func(t *testing.T) {
		expected, _ := time.Parse(tmdb.ReleaseDateFormat, "2023-11-25")
		got, err := parseTMDBDate("2023-11-25")
		require.NoError(t, err)
		assert.Equal(t, &expected, got)
	})

	t.Run("returns error for invalid date", func(t *testing.T) {
		_, err := parseTMDBDate("25-11-2023")
		require.Error(t, err)
	})
}

func TestParseExternalIDs(t *testing.T) {
	t.Run("parses valid JSON", func(t *testing.T) {
		resp := &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewBufferString(`{"imdb_id":"tt1234567","tvdb_id":999}`))}
		got, err := parseExternalIDs(resp)
		require.NoError(t, err)
		assert.Equal(t, &ExternalIDsData{ImdbID: ptr.To("tt1234567"), TvdbID: ptr.To(999)}, got)
	})

	t.Run("returns error on invalid JSON", func(t *testing.T) {
		resp := &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewBufferString(`not json`))}
		_, err := parseExternalIDs(resp)
		require.Error(t, err)
	})

	t.Run("returns error on body read failure", func(t *testing.T) {
		resp := &http.Response{StatusCode: 200, Body: io.NopCloser(iotest.ErrReader(errors.New("read error")))}
		_, err := parseExternalIDs(resp)
		require.Error(t, err)
	})

	t.Run("handles missing fields", func(t *testing.T) {
		resp := &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewBufferString(`{"tvdb_id":42}`))}
		got, err := parseExternalIDs(resp)
		require.NoError(t, err)
		assert.Equal(t, &ExternalIDsData{TvdbID: ptr.To(42)}, got)
	})
}

func TestParseWatchProviders(t *testing.T) {
	t.Run("parses US flatrate providers", func(t *testing.T) {
		body := `{"results":{"US":{"flatrate":[{"provider_id":8,"provider_name":"Netflix","logo_path":"/n.png"}],"link":"https://example.com"}}}`
		resp := &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewBufferString(body))}
		got, err := parseWatchProviders(resp)
		require.NoError(t, err)
		assert.Equal(t, &WatchProvidersData{
			US: WatchProviderRegionData{
				Flatrate: []WatchProviderData{{ProviderID: 8, Name: "Netflix", LogoPath: ptr.To("/n.png")}},
				Link:     ptr.To("https://example.com"),
			},
		}, got)
	})

	t.Run("returns empty result when no US region", func(t *testing.T) {
		body := `{"results":{"GB":{"flatrate":[{"provider_id":8,"provider_name":"Netflix"}]}}}`
		resp := &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewBufferString(body))}
		got, err := parseWatchProviders(resp)
		require.NoError(t, err)
		assert.Equal(t, &WatchProvidersData{}, got)
	})

	t.Run("skips providers missing id or name", func(t *testing.T) {
		body := `{"results":{"US":{"flatrate":[{"logo_path":"/x.png"},{"provider_id":10,"provider_name":"Disney+"}]}}}`
		resp := &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewBufferString(body))}
		got, err := parseWatchProviders(resp)
		require.NoError(t, err)
		assert.Equal(t, &WatchProvidersData{
			US: WatchProviderRegionData{
				Flatrate: []WatchProviderData{{ProviderID: 10, Name: "Disney+"}},
			},
		}, got)
	})

	t.Run("returns error on invalid JSON", func(t *testing.T) {
		resp := &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewBufferString(`{bad json}`))}
		_, err := parseWatchProviders(resp)
		require.Error(t, err)
	})

	t.Run("returns error on body read failure", func(t *testing.T) {
		resp := &http.Response{StatusCode: 200, Body: io.NopCloser(iotest.ErrReader(errors.New("read error")))}
		_, err := parseWatchProviders(resp)
		require.Error(t, err)
	})
}
