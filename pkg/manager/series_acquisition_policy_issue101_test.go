package manager

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/kasuboski/mediaz/config"
	"github.com/kasuboski/mediaz/pkg/download"
	downloadMock "github.com/kasuboski/mediaz/pkg/download/mocks"
	"github.com/kasuboski/mediaz/pkg/indexer"
	"github.com/kasuboski/mediaz/pkg/ptr"
	"github.com/kasuboski/mediaz/pkg/storage"
	sqlcdb "github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/oapi-codegen/nullable"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// issue101TVFixtures builds an in-memory library: series (movie-scope profile 1)
// with season 1 containing the given episodes (number, monitored, profile override).
func issue101TVFixtures(t *testing.T, ctx context.Context, store storage.Storage, episodes []struct {
	number    int64
	monitored int64
	override  sql.NullInt64
}) (sqlcdb.Series, sqlcdb.ListSeasonsRow, []sqlcdb.ListEpisodesRow) {
	t.Helper()

	// Two download clients so transition rows can reference client id 2.
	for i := 0; i < 2; i++ {
		_, err := store.CreateDownloadClient(ctx, sqlcdb.DownloadClient{ID: int64(i + 1), Implementation: "transmission", Type: "torrent", Host: "transmission", Port: 8080, Scheme: "http"})
		require.NoError(t, err)
	}

	seriesMetadataID, err := store.CreateSeriesMetadata(ctx, sqlcdb.SeriesMetadatum{TmdbID: 1, Title: "Series", Status: "Continuing"})
	require.NoError(t, err)
	seriesID, err := store.CreateSeries(ctx, sqlcdb.Series{QualityProfileID: 1, Monitored: 1, SeriesMetadataID: sql.NullInt64{Int64: seriesMetadataID, Valid: true}}, storage.SeriesStateMissing)
	require.NoError(t, err)
	seasonMetadataID, err := store.CreateSeasonMetadata(ctx, sqlcdb.SeasonMetadatum{SeriesMetadataID: seriesMetadataID, Title: "Season 1", Number: 1})
	require.NoError(t, err)
	seasonID, err := store.CreateSeason(ctx, sqlcdb.Season{SeriesID: seriesID, SeasonNumber: 1, SeasonMetadataID: sql.NullInt64{Int64: seasonMetadataID, Valid: true}, Monitored: 1}, storage.SeasonStateMissing)
	require.NoError(t, err)

	rows := make([]sqlcdb.ListEpisodesRow, 0, len(episodes))
	for _, e := range episodes {
		metaID, err := store.CreateEpisodeMetadata(ctx, sqlcdb.EpisodeMetadatum{
			TmdbID:           e.number,
			Title:            "Episode",
			Number:           e.number,
			SeasonMetadataID: seasonMetadataID,
			AirDate:          sql.NullTime{Time: time.Now().Add(-2 * time.Hour), Valid: true},
			Runtime:          sql.NullInt64{Int64: 42, Valid: true},
		})
		require.NoError(t, err)
		episodeID, err := store.CreateEpisode(ctx, sqlcdb.Episode{
			SeasonID:          seasonID,
			EpisodeNumber:     e.number,
			Monitored:         e.monitored,
			QualityProfileID:  e.override,
			EpisodeMetadataID: sql.NullInt64{Int64: metaID, Valid: true},
		}, storage.EpisodeStateMissing)
		require.NoError(t, err)
		rows = append(rows, sqlcdb.ListEpisodesRow{
			Episode: sqlcdb.Episode{ID: episodeID, SeasonID: seasonID, EpisodeNumber: e.number, Monitored: e.monitored, QualityProfileID: e.override, EpisodeMetadataID: sql.NullInt64{Int64: metaID, Valid: true}},
			State:   string(storage.EpisodeStateMissing),
		})
	}

	series, err := store.GetSeriesByID(ctx, seriesID)
	require.NoError(t, err)
	season, err := store.GetSeasonByID(ctx, seasonID)
	require.NoError(t, err)

	return series.Series, seasonListRow(season), rows
}

func issue101EpisodeRelease(n int64, title string) *indexer.Release {
	return &indexer.Release{
		ID:       ptr.To(int32(n)),
		Title:    nullable.NewNullableWithValue(title),
		Size:     sizeGBToBytes(2),
		Protocol: ptr.To(string(indexer.DownloadProtocolTorrent)),
	}
}

func issue101ProfileSeed() []sqlcdb.ListQualityProfileRowsRow {
	return []sqlcdb.ListQualityProfileRowsRow{{QualityDefinition: sqlcdb.QualityDefinition{
		Name: "HD", MinSize: 10, MaxSize: 1000, PreferredSize: 995, MediaType: "episode",
	}}}
}

func TestResolveTVQualityProfileID_Issue101(t *testing.T) {
	t.Run("episode overrides season overrides series", func(t *testing.T) {
		id, source := ResolveTVQualityProfileID(1, sql.NullInt64{Int64: 4, Valid: true}, sql.NullInt64{Int64: 5, Valid: true})
		assert.Equal(t, int64(5), id)
		assert.Equal(t, "episode", source)

		id, source = ResolveTVQualityProfileID(1, sql.NullInt64{Int64: 4, Valid: true}, sql.NullInt64{})
		assert.Equal(t, int64(4), id)
		assert.Equal(t, "season", source)

		id, source = ResolveTVQualityProfileID(1, sql.NullInt64{}, sql.NullInt64{})
		assert.Equal(t, int64(1), id)
		assert.Equal(t, "series", source)
	})

	t.Run("invalid overrides are ignored, not silently substituted", func(t *testing.T) {
		id, source := ResolveTVQualityProfileID(2, sql.NullInt64{Int64: 0, Valid: true}, sql.NullInt64{Int64: -3, Valid: true})
		assert.Equal(t, int64(2), id)
		assert.Equal(t, "series", source)
	})
}

func TestTVProfileResolver_Issue101(t *testing.T) {
	t.Run("rejects missing profile without fallback", func(t *testing.T) {
		resolver := newTVProfileResolver(context.Background(), nil, 0, nil)
		_, _, _, err := resolver.resolve(0, sql.NullInt64{}, sql.NullInt64{})
		assert.Error(t, err)
	})

	t.Run("rejects movie profile for episode content", func(t *testing.T) {
		movieRows := []sqlcdb.ListQualityProfileRowsRow{{QualityDefinition: sqlcdb.QualityDefinition{Name: "HD", MinSize: 15, MaxSize: 1000, MediaType: "movie"}}}
		resolver := newTVProfileResolver(context.Background(), nil, 1, movieRows)
		_, _, _, err := resolver.resolve(1, sql.NullInt64{}, sql.NullInt64{})
		assert.ErrorContains(t, err, "not episode-compatible")
	})

	t.Run("episode override fetches episode-compatible profile", func(t *testing.T) {
		ctx := context.Background()
		store := newStore(t, ctx)
		resolver := newTVProfileResolver(ctx, nil, 0, nil)
		resolver.m = &MediaManager{qualityService: NewQualityService(store)}
		rows, id, source, err := resolver.resolve(1, sql.NullInt64{}, sql.NullInt64{Int64: 5, Valid: true})
		require.NoError(t, err)
		assert.Equal(t, int64(5), id)
		assert.Equal(t, "episode", source)
		require.NotEmpty(t, rows)
		assert.Equal(t, "episode", rows[0].QualityDefinition.MediaType)
	})
}

func TestLooksLikeMultiEpisodeRelease_Issue101(t *testing.T) {
	assert.False(t, looksLikeMultiEpisodeRelease("Series.S01E01.1080p.WEB-DL-GROUP"))
	assert.False(t, looksLikeMultiEpisodeRelease("Series.1x01.1080p.WEB-DL-GROUP"))
	assert.True(t, looksLikeMultiEpisodeRelease("Series.S01E01-E02.1080p.WEB-DL-GROUP"))
	assert.True(t, looksLikeMultiEpisodeRelease("Series.S01E01E02.1080p.WEB-DL-GROUP"))
	assert.True(t, looksLikeMultiEpisodeRelease("Series.S01E01+E03.1080p.WEB-DL-GROUP"))
	assert.True(t, looksLikeMultiEpisodeRelease("Series.S01E01.S01E02.1080p.WEB-DL-GROUP"))
	assert.True(t, looksLikeMultiEpisodeRelease("Series.1x01-1x02.1080p.WEB-DL-GROUP"))
	assert.True(t, looksLikeMultiEpisodeRelease(""))
}

func TestSeasonPackEligibility_Issue101(t *testing.T) {
	ctx := context.Background()
	aired := func(sqlcdb.ListEpisodesRow) bool { return true }

	setup := func(t *testing.T) (*tvProfileResolver, sqlcdb.Series, sqlcdb.ListSeasonsRow) {
		store := newStore(t, ctx)
		series, season, _ := issue101TVFixtures(t, ctx, store, []struct {
			number    int64
			monitored int64
			override  sql.NullInt64
		}{})
		resolver := newTVProfileResolver(ctx, nil, 1, issue101ProfileSeed())
		resolver.m = &MediaManager{qualityService: NewQualityService(store)}
		return resolver, series, season
	}

	t.Run("eligible when every known episode is selected missing aired and homogeneous", func(t *testing.T) {
		resolver, series, season := setup(t)
		episodes := []sqlcdb.ListEpisodesRow{
			{Episode: sqlcdb.Episode{ID: 1, EpisodeNumber: 1, Monitored: 1}, State: string(storage.EpisodeStateMissing)},
			{Episode: sqlcdb.Episode{ID: 2, EpisodeNumber: 2, Monitored: 1}, State: string(storage.EpisodeStateMissing)},
		}
		d := resolver.evaluateSeasonPackEligibility(series.QualityProfileID, season.Season.QualityProfileID, season.Season, episodes, aired)
		assert.True(t, d.eligible)
		assert.Equal(t, int64(1), d.profileID)
		assert.Equal(t, []int64{1, 2}, d.episodeIDs)
	})

	t.Run("rejects partial monitoring", func(t *testing.T) {
		resolver, series, season := setup(t)
		episodes := []sqlcdb.ListEpisodesRow{
			{Episode: sqlcdb.Episode{ID: 1, EpisodeNumber: 1, Monitored: 1}, State: string(storage.EpisodeStateMissing)},
			{Episode: sqlcdb.Episode{ID: 2, EpisodeNumber: 2, Monitored: 0}, State: string(storage.EpisodeStateMissing)},
		}
		d := resolver.evaluateSeasonPackEligibility(series.QualityProfileID, season.Season.QualityProfileID, season.Season, episodes, aired)
		assert.False(t, d.eligible)
		assert.Contains(t, d.reason, "not monitored")
	})

	t.Run("rejects non-missing or active episodes", func(t *testing.T) {
		resolver, series, season := setup(t)
		d := resolver.evaluateSeasonPackEligibility(series.QualityProfileID, season.Season.QualityProfileID, season.Season, []sqlcdb.ListEpisodesRow{
			{Episode: sqlcdb.Episode{ID: 1, EpisodeNumber: 1, Monitored: 1}, State: string(storage.EpisodeStateDownloading)},
		}, aired)
		assert.False(t, d.eligible)

		d = resolver.evaluateSeasonPackEligibility(series.QualityProfileID, season.Season.QualityProfileID, season.Season, []sqlcdb.ListEpisodesRow{
			{Episode: sqlcdb.Episode{ID: 1, EpisodeNumber: 1, Monitored: 1}, State: string(storage.EpisodeStateMissing), DownloadID: sql.NullString{String: "9", Valid: true}},
		}, aired)
		assert.False(t, d.eligible)
		assert.Contains(t, d.reason, "active download")
	})

	t.Run("rejects unaired episodes and missing metadata", func(t *testing.T) {
		resolver, series, season := setup(t)
		d := resolver.evaluateSeasonPackEligibility(series.QualityProfileID, season.Season.QualityProfileID, season.Season, []sqlcdb.ListEpisodesRow{
			{Episode: sqlcdb.Episode{ID: 1, EpisodeNumber: 1, Monitored: 1}, State: string(storage.EpisodeStateMissing)},
		}, func(sqlcdb.ListEpisodesRow) bool { return false })
		assert.False(t, d.eligible)
	})

	t.Run("rejects heterogeneous effective profiles", func(t *testing.T) {
		resolver, series, season := setup(t)
		episodes := []sqlcdb.ListEpisodesRow{
			{Episode: sqlcdb.Episode{ID: 1, EpisodeNumber: 1, Monitored: 1, QualityProfileID: sql.NullInt64{Int64: 5, Valid: true}}, State: string(storage.EpisodeStateMissing)},
			{Episode: sqlcdb.Episode{ID: 2, EpisodeNumber: 2, Monitored: 1, QualityProfileID: sql.NullInt64{Int64: 6, Valid: true}}, State: string(storage.EpisodeStateMissing)},
		}
		d := resolver.evaluateSeasonPackEligibility(series.QualityProfileID, season.Season.QualityProfileID, season.Season, episodes, aired)
		assert.False(t, d.eligible)
		assert.Contains(t, d.reason, "different quality profile")
	})
}

func TestReconcileMissingSeasonWithMetadata_Issue101(t *testing.T) {
	ctx := context.Background()

	runReconcile := func(t *testing.T, store storage.Storage, series sqlcdb.Series, season sqlcdb.ListSeasonsRow, releases []*indexer.Release, expectedAdds int, seed []sqlcdb.ListQualityProfileRowsRow) {
		t.Helper()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		client := downloadMock.NewMockDownloadClient(ctrl)
		factory := downloadMock.NewMockFactory(ctrl)
		clientModel := sqlcdb.DownloadClient{ID: 2, Implementation: "transmission", Type: "torrent", Port: 8080, Host: "transmission", Scheme: "http"}
		if expectedAdds > 0 {
			factory.EXPECT().NewDownloadClient(clientModel).Times(expectedAdds).Return(client, nil)
			client.EXPECT().Add(ctx, gomock.Any()).Times(expectedAdds).Return(download.Status{ID: "123", Name: "test download"}, nil)
		}
		snapshot := newReconcileSnapshot([]sqlcdb.Indexer{{ID: 1}}, []sqlcdb.DownloadClient{clientModel})
		m := New(nil, nil, nil, store, factory, config.Manager{}, config.Config{})
		require.NoError(t, m.reconcileMissingSeasonWithMetadata(ctx, "Series", series, season, nil, nil, nil, snapshot, seed, releases))
	}

	t.Run("pack acquired when whole season is eligible", func(t *testing.T) {
		store := newStore(t, ctx)
		series, season, _ := issue101TVFixtures(t, ctx, store, []struct {
			number    int64
			monitored int64
			override  sql.NullInt64
		}{
			{number: 1, monitored: 1},
			{number: 2, monitored: 1},
		})
		releases := []*indexer.Release{issue101EpisodeRelease(1, "Series.S01.1080p.WEB-DL-GROUP")}
		runReconcile(t, store, series, season, releases, 1, issue101ProfileSeed())

		episodes, err := store.ListEpisodes(ctx, storage.EpisodeFilter{})
		require.NoError(t, err)
		require.Len(t, episodes, 2)
		for _, e := range episodes {
			assert.Equal(t, string(storage.EpisodeStateDownloading), e.State)
			assert.Equal(t, sql.NullBool{Bool: true, Valid: true}, e.IsEntireSeasonDownload)
		}
	})

	t.Run("pack rejected for unmonitored sibling falls back to monitored individual episode only", func(t *testing.T) {
		store := newStore(t, ctx)
		series, season, _ := issue101TVFixtures(t, ctx, store, []struct {
			number    int64
			monitored int64
			override  sql.NullInt64
		}{
			{number: 1, monitored: 1},
			{number: 2, monitored: 0},
		})
		releases := []*indexer.Release{
			issue101EpisodeRelease(1, "Series.S01.1080p.WEB-DL-GROUP"),
			issue101EpisodeRelease(2, "Series.S01E01.1080p.WEB-DL-GROUP"),
		}
		runReconcile(t, store, series, season, releases, 1, issue101ProfileSeed())

		episodes, err := store.ListEpisodes(ctx, storage.EpisodeFilter{})
		require.NoError(t, err)
		require.Len(t, episodes, 2)
		for _, e := range episodes {
			if e.Episode.Monitored == 0 {
				assert.Equal(t, string(storage.EpisodeStateMissing), e.State)
				assert.Equal(t, sql.NullString{}, e.DownloadID)
				continue
			}
			assert.Equal(t, string(storage.EpisodeStateDownloading), e.State)
			assert.Equal(t, sql.NullBool{}, e.IsEntireSeasonDownload)
		}
	})

	t.Run("multi-episode release covering unrequested sibling is never used from single-episode path", func(t *testing.T) {
		store := newStore(t, ctx)
		series, season, _ := issue101TVFixtures(t, ctx, store, []struct {
			number    int64
			monitored int64
			override  sql.NullInt64
		}{
			{number: 1, monitored: 1},
			{number: 2, monitored: 0},
		})
		releases := []*indexer.Release{
			issue101EpisodeRelease(1, "Series.S01E01-E02.1080p.WEB-DL-GROUP"),
			issue101EpisodeRelease(2, "Series.S01E01.1080p.WEB-DL-GROUP"),
		}
		runReconcile(t, store, series, season, releases, 1, issue101ProfileSeed())

		episodes, err := store.ListEpisodes(ctx, storage.EpisodeFilter{})
		require.NoError(t, err)
		require.Len(t, episodes, 2)
		for _, e := range episodes {
			if e.Episode.Monitored == 1 {
				assert.Equal(t, string(storage.EpisodeStateDownloading), e.State)
			} else {
				assert.Equal(t, string(storage.EpisodeStateMissing), e.State)
			}
		}
	})

	t.Run("heterogeneous episode profiles reject pack but individual fallback uses effective profiles", func(t *testing.T) {
		store := newStore(t, ctx)
		series, season, _ := issue101TVFixtures(t, ctx, store, []struct {
			number    int64
			monitored int64
			override  sql.NullInt64
		}{
			{number: 1, monitored: 1, override: sql.NullInt64{Int64: 5, Valid: true}},
			{number: 2, monitored: 1, override: sql.NullInt64{Int64: 6, Valid: true}},
		})
		releases := []*indexer.Release{
			issue101EpisodeRelease(1, "Series.S01.1080p.WEB-DL-GROUP"),
			issue101EpisodeRelease(2, "Series.S01E01.1080p.WEB-DL-GROUP"),
			issue101EpisodeRelease(3, "Series.S01E02.1080p.WEB-DL-GROUP"),
		}
		runReconcile(t, store, series, season, releases, 2, issue101ProfileSeed())

		episodes, err := store.ListEpisodes(ctx, storage.EpisodeFilter{})
		require.NoError(t, err)
		require.Len(t, episodes, 2)
		for _, e := range episodes {
			assert.Equal(t, string(storage.EpisodeStateDownloading), e.State)
			assert.Equal(t, sql.NullBool{}, e.IsEntireSeasonDownload)
		}
	})

	t.Run("season profile override is used when episodes inherit", func(t *testing.T) {
		store := newStore(t, ctx)
		series, season, _ := issue101TVFixtures(t, ctx, store, []struct {
			number    int64
			monitored int64
			override  sql.NullInt64
		}{
			{number: 1, monitored: 1},
		})
		// Season override to an episode-compatible profile (5).
		require.NoError(t, store.UpdateSeasonQualityProfile(ctx, season.Season.ID, sql.NullInt64{Int64: 5, Valid: true}))
		season.Season.QualityProfileID = sql.NullInt64{Int64: 5, Valid: true}

		releases := []*indexer.Release{issue101EpisodeRelease(1, "Series.S01E01.1080p.WEB-DL-GROUP")}
		runReconcile(t, store, series, season, releases, 1, issue101ProfileSeed())

		episodes, err := store.ListEpisodes(ctx, storage.EpisodeFilter{})
		require.NoError(t, err)
		require.Len(t, episodes, 1)
		assert.Equal(t, string(storage.EpisodeStateDownloading), episodes[0].State)
	})

	t.Run("movie-scope series profile without valid effective profile acquires nothing", func(t *testing.T) {
		// The seed is the movie-scope profile: resolution must not fall back
		// to it or any other profile, so nothing is enqueued.
		store := newStore(t, ctx)
		series, season, _ := issue101TVFixtures(t, ctx, store, []struct {
			number    int64
			monitored int64
			override  sql.NullInt64
		}{
			{number: 1, monitored: 1},
		})
		releases := []*indexer.Release{issue101EpisodeRelease(1, "Series.S01E01.1080p.WEB-DL-GROUP")}
		runReconcile(t, store, series, season, releases, 0, []sqlcdb.ListQualityProfileRowsRow{{QualityDefinition: sqlcdb.QualityDefinition{Name: "HD", MinSize: 15, MaxSize: 1000, MediaType: "movie"}}})

		episodes, err := store.ListEpisodes(ctx, storage.EpisodeFilter{})
		require.NoError(t, err)
		require.Len(t, episodes, 1)
		assert.Equal(t, string(storage.EpisodeStateMissing), episodes[0].State)
	})
}

func TestReconcileMissingEpisodeWithMetadata_Issue101(t *testing.T) {
	ctx := context.Background()

	t.Run("explicit request bypasses monitoring for exactly the target", func(t *testing.T) {
		store := newStore(t, ctx)
		series, _, episodes := issue101TVFixtures(t, ctx, store, []struct {
			number    int64
			monitored int64
			override  sql.NullInt64
		}{
			{number: 1, monitored: 0},
			{number: 2, monitored: 0},
		})
		require.Equal(t, int64(1), series.QualityProfileID)

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		client := downloadMock.NewMockDownloadClient(ctrl)
		factory := downloadMock.NewMockFactory(ctrl)
		clientModel := sqlcdb.DownloadClient{ID: 2, Implementation: "transmission", Type: "torrent", Port: 8080, Host: "transmission", Scheme: "http"}
		factory.EXPECT().NewDownloadClient(clientModel).Return(client, nil)
		client.EXPECT().Add(ctx, gomock.Any()).Return(download.Status{ID: "123", Name: "test download"}, nil)
		snapshot := newReconcileSnapshot([]sqlcdb.Indexer{{ID: 1}}, []sqlcdb.DownloadClient{clientModel})

		m := New(nil, nil, nil, store, factory, config.Manager{}, config.Config{})
		target := episodes[0]
		// Episode override to an episode-compatible profile; the series
		// profile itself is movie-scope and must never be used as fallback.
		target.Episode.QualityProfileID = sql.NullInt64{Int64: 5, Valid: true}
		require.NoError(t, store.UpdateEpisodeQualityProfile(ctx, target.Episode.ID, target.Episode.QualityProfileID))
		updated, err := m.reconcileMissingEpisodeWithMetadata(ctx, "Series", series.QualityProfileID, sql.NullInt64{}, 1, target, nil, snapshot, nil, []*indexer.Release{
			issue101EpisodeRelease(1, "Series.S01E01.1080p.WEB-DL-GROUP"),
		})
		require.NoError(t, err)
		assert.True(t, updated)

		got, err := store.GetEpisodeByID(ctx, target.Episode.ID)
		require.NoError(t, err)
		assert.Equal(t, sql.NullString{String: string(storage.EpisodeStateDownloading), Valid: true}, got.State)
		assert.Equal(t, int64(0), got.Episode.Monitored)

		// Sibling untouched.
		other, err := store.GetEpisodeByID(ctx, episodes[1].Episode.ID)
		require.NoError(t, err)
		assert.Equal(t, sql.NullString{String: string(storage.EpisodeStateMissing), Valid: true}, other.State)
	})

	t.Run("non-missing target is never re-enqueued", func(t *testing.T) {
		store := newStore(t, ctx)
		series, _, episodes := issue101TVFixtures(t, ctx, store, []struct {
			number    int64
			monitored int64
			override  sql.NullInt64
		}{
			{number: 1, monitored: 1},
		})

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		snapshot := newReconcileSnapshot([]sqlcdb.Indexer{{ID: 1}}, nil)
		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})

		target := episodes[0]
		target.State = string(storage.EpisodeStateDownloading)
		target.DownloadID = sql.NullString{String: "77", Valid: true}
		updated, err := m.reconcileMissingEpisodeWithMetadata(ctx, "Series", series.QualityProfileID, sql.NullInt64{}, 1, target, nil, snapshot, issue101ProfileSeed(), []*indexer.Release{
			issue101EpisodeRelease(1, "Series.S01E01.1080p.WEB-DL-GROUP"),
		})
		require.NoError(t, err)
		assert.False(t, updated)
	})
}
