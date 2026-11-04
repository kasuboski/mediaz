package manager

import (
	"context"
	"database/sql"
	"fmt"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/kasuboski/mediaz/config"
	"github.com/kasuboski/mediaz/internal/testsupport/faketmdb"
	"github.com/kasuboski/mediaz/pkg/ptr"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite"
	"github.com/kasuboski/mediaz/pkg/tmdb"
	"github.com/stretchr/testify/require"
)

func fakeTMDBTVManager(t *testing.T, fake *faketmdb.Server) (MediaManager, context.Context) {
	t.Helper()
	ctx := context.Background()
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)
	client, err := tmdb.New(server.URL, "integration-test-token")
	require.NoError(t, err)
	store, err := sqlite.New(ctx, ":memory:")
	require.NoError(t, err)
	return New(client, nil, nil, store, nil, config.Manager{}, config.Config{}), ctx
}

func TestFakeTMDBTVSearchMetadataHierarchyPersistence(t *testing.T) {
	fake := faketmdb.New()
	m, ctx := fakeTMDBTVManager(t, fake)
	search, err := m.SearchTV(ctx, "Lanterns at Low Tide", 1)
	require.NoError(t, err)
	require.Len(t, search.Results, 1)
	require.Equal(t, int64(2001), int64(*search.Results[0].ID))

	metadata, err := m.GetSeriesMetadata(ctx, 2001)
	require.NoError(t, err)
	require.Equal(t, "Lanterns at Low Tide", metadata.Title)
	require.Equal(t, sqlcdb.SeriesMetadatum{ID: metadata.ID, TmdbID: 2001, Title: "Lanterns at Low Tide", Overview: sql.NullString{String: "Synthetic ensemble series: Lanterns at Low Tide", Valid: true}, FirstAirDate: sql.NullTime{Time: time.Date(2018, 9, 1, 0, 0, 0, 0, time.UTC), Valid: true}, SeasonCount: 2, EpisodeCount: 4, Status: "Ended", PosterPath: sql.NullString{String: "/synthetic/tv/2001.jpg", Valid: true}, ExternalIds: sql.NullString{String: `{"imdb_id":"tt80002001","tvdb_id":702001}`, Valid: true}, WatchProviders: sql.NullString{String: `{"US":{"flatrate":[{"provider_id":8,"provider_name":"Netflix","logo_path":"/synthetic/providers/netflix.jpg"}],"link":"https://example.invalid/watch"}}`, Valid: true}}, *metadata)

	seasons, err := m.seriesMetaStorage.ListSeasonMetadataBySeriesMetadataID(ctx, metadata.ID)
	require.NoError(t, err)
	require.Len(t, seasons, 2)
	require.Equal(t, []sqlcdb.SeasonMetadatum{
		{ID: seasons[0].ID, SeriesMetadataID: metadata.ID, Number: 0, TmdbID: 20010, Title: "Specials", Overview: sql.NullString{String: "Synthetic season.", Valid: true}, AirDate: sql.NullTime{Time: time.Date(2018, 1, 15, 0, 0, 0, 0, time.UTC), Valid: true}},
		{ID: seasons[1].ID, SeriesMetadataID: metadata.ID, Number: 1, TmdbID: 20011, Title: "Season 1", Overview: sql.NullString{String: "Synthetic season.", Valid: true}, AirDate: sql.NullTime{Time: time.Date(2019, 1, 15, 0, 0, 0, 0, time.UTC), Valid: true}},
	}, seasons)
	for i, season := range seasons {
		episodes, err := m.seriesMetaStorage.ListEpisodeMetadataBySeasonMetadataID(ctx, season.ID)
		require.NoError(t, err)
		year, month := 2018, time.January
		ids := []int64{200101, 200102}
		if i == 1 {
			year, month, ids = 2019, time.February, []int64{200111, 200112}
		}
		want := make([]sqlcdb.EpisodeMetadatum, 2)
		for j := range want {
			want[j] = sqlcdb.EpisodeMetadatum{SeasonMetadataID: season.ID, Number: int64(j + 1), TmdbID: ids[j], Title: "Episode " + strconv.Itoa(j+1), Overview: sql.NullString{String: "Synthetic episode " + strconv.Itoa(j+1) + ".", Valid: true}, AirDate: sql.NullTime{Time: time.Date(year, month, 11+j, 0, 0, 0, 0, time.UTC), Valid: true}, Runtime: sql.NullInt64{Int64: 43, Valid: true}, StillPath: sql.NullString{String: fmt.Sprintf("/synthetic/episodes/%d.jpg", ids[j]), Valid: true}}
		}
		for j := range want {
			want[j].ID = episodes[j].ID
		}
		require.Equal(t, want, episodes)
	}

	profile, err := m.AddQualityProfile(ctx, AddQualityProfileRequest{Name: "tv-integration", QualityIDs: []int64{17, 18, 19, 20, 21}, CutoffQualityID: ptr.To(int64(19))})
	require.NoError(t, err)
	added, err := m.AddSeriesToLibrary(ctx, AddSeriesRequest{TMDBID: 2001, QualityProfileID: profile.ID})
	require.NoError(t, err)
	librarySeasons, err := m.seriesStorage.ListSeasons(ctx, storage.SeasonFilter{SeriesID: ptr.To(added.Series.ID)})
	require.NoError(t, err)
	require.Len(t, librarySeasons, 2)
	for _, season := range librarySeasons {
		require.Equal(t, added.Series.ID, season.Season.SeriesID)
		libraryEpisodes, err := m.seriesStorage.ListEpisodes(ctx, storage.EpisodeFilter{SeasonID: ptr.To(season.Season.ID)})
		require.NoError(t, err)
		require.Len(t, libraryEpisodes, map[int64]int{0: 2, 1: 2}[season.Season.SeasonNumber])
		for _, episode := range libraryEpisodes {
			require.Equal(t, season.Season.ID, episode.Episode.SeasonID)
		}
	}
	detail, err := m.GetTVDetailByTMDBID(ctx, 2001)
	require.NoError(t, err)
	require.Equal(t, int64(2001), detail.TMDBID)
	require.Equal(t, ptr.To("/synthetic/tv/2001-backdrop.jpg"), detail.BackdropPath)
	require.Empty(t, detail.Networks) // The TV2001 fixture does not define networks.
	require.Equal(t, []string{"Drama"}, detail.Genres)
	require.Nil(t, detail.OriginalLanguage) // The TV2001 fixture does not define an original language.
	require.Equal(t, ptr.To("Ended"), detail.Status)
	require.Equal(t, []WatchProvider{{ProviderID: 8, Name: "Netflix", LogoPath: ptr.To("/synthetic/providers/netflix.jpg")}}, detail.WatchProviders)
	require.Equal(t, &ExternalIDs{ImdbID: ptr.To("tt80002001"), TvdbID: ptr.To(702001)}, detail.ExternalIDs)

	requests := fake.Requests()
	var routeCounts = map[string]int{}
	for _, request := range requests {
		require.Equal(t, "GET", request.Method)
		routeCounts[request.Path]++
	}
	require.Equal(t, 1, routeCounts["/3/search/tv"])
	require.Equal(t, 2, routeCounts["/3/tv/2001"])
	require.Equal(t, 1, routeCounts["/3/tv/2001/season/0"])
	require.Equal(t, 1, routeCounts["/3/tv/2001/season/1"])
	require.Equal(t, 1, routeCounts["/3/tv/2001/external_ids"])
	require.Equal(t, 1, routeCounts["/3/tv/2001/watch/providers"])
}

func TestFakeTMDBTVFixtureAbsentProviders(t *testing.T) {
	m, ctx := fakeTMDBTVManager(t, faketmdb.New())
	metadata, err := m.GetSeriesMetadata(ctx, 2002)
	require.NoError(t, err)
	require.Equal(t, "The Astral Workshop", metadata.Title)
	require.True(t, metadata.ExternalIds.Valid)
	require.JSONEq(t, `{"imdb_id":"tt80002002","tvdb_id":702002}`, metadata.ExternalIds.String)
	require.True(t, metadata.WatchProviders.Valid)
	seasons, err := m.seriesMetaStorage.ListSeasonMetadataBySeriesMetadataID(ctx, metadata.ID)
	require.NoError(t, err)
	require.Len(t, seasons, 2)
	for _, season := range seasons {
		require.Equal(t, metadata.ID, season.SeriesMetadataID)
		episodes, err := m.seriesMetaStorage.ListEpisodeMetadataBySeasonMetadataID(ctx, season.ID)
		require.NoError(t, err)
		require.Len(t, episodes, 2)
		for _, episode := range episodes {
			require.Equal(t, season.ID, episode.SeasonMetadataID)
		}
	}
	persisted, err := m.GetTVDetailByTMDBID(ctx, 2002)
	require.NoError(t, err)
	require.Equal(t, int64(2002), persisted.TMDBID)
	require.Equal(t, &ExternalIDs{ImdbID: ptr.To("tt80002002"), TvdbID: ptr.To(702002)}, persisted.ExternalIDs)
	require.Empty(t, persisted.WatchProviders)
}

func TestFakeTMDBTVAbsentProvidersAndBestEffortEnrichment(t *testing.T) {
	fake := faketmdb.New()
	fake.SetStatus("/3/tv/2002/external_ids", 500)
	fake.SetStatus("/3/tv/2002/watch/providers", 500)
	m, ctx := fakeTMDBTVManager(t, fake)
	metadata, err := m.GetSeriesMetadata(ctx, 2002)
	require.NoError(t, err)
	require.Equal(t, "The Astral Workshop", metadata.Title)
	require.Equal(t, "Returning Series", metadata.Status)
	require.False(t, metadata.ExternalIds.Valid)
	require.False(t, metadata.WatchProviders.Valid)
	seasons, err := m.seriesMetaStorage.ListSeasonMetadataBySeriesMetadataID(ctx, metadata.ID)
	require.NoError(t, err)
	require.Len(t, seasons, 2)
	for _, season := range seasons {
		require.Equal(t, metadata.ID, season.SeriesMetadataID)
		episodes, err := m.seriesMetaStorage.ListEpisodeMetadataBySeasonMetadataID(ctx, season.ID)
		require.NoError(t, err)
		require.Len(t, episodes, 2)
		for _, episode := range episodes {
			require.Equal(t, season.ID, episode.SeasonMetadataID)
		}
	}
	persisted, err := m.GetTVDetailByTMDBID(ctx, 2002)
	require.NoError(t, err)
	require.Equal(t, int64(2002), persisted.TMDBID)
	require.Nil(t, persisted.ExternalIDs)
	require.Empty(t, persisted.WatchProviders)
}
