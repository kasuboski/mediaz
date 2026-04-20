package manager

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"
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
