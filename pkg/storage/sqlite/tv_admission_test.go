package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/stretchr/testify/require"
)

func TestTVAdmissionSurvivesRestart(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "admission.db")
	store, err := New(ctx, path)
	require.NoError(t, err)
	series, err := store.CreateSeries(ctx, sqlcdb.Series{QualityProfileID: 1, Monitored: 1}, storage.SeriesStateMissing)
	require.NoError(t, err)
	season, err := store.CreateSeason(ctx, sqlcdb.Season{SeriesID: series, SeasonNumber: 1, Monitored: 1}, storage.SeasonStateMissing)
	require.NoError(t, err)
	a, err := store.CreateEpisode(ctx, sqlcdb.Episode{SeasonID: season, EpisodeNumber: 1, Monitored: 1}, storage.EpisodeStateMissing)
	require.NoError(t, err)
	b, err := store.CreateEpisode(ctx, sqlcdb.Episode{SeasonID: season, EpisodeNumber: 2, Monitored: 1}, storage.EpisodeStateMissing)
	require.NoError(t, err)
	require.NoError(t, store.ReserveTVEpisodes(ctx, []int64{a, b}))
	require.NoError(t, store.(*SQLite).db.Close())
	reopened, err := New(ctx, path)
	require.NoError(t, err)
	defer reopened.(*SQLite).db.Close()
	require.ErrorContains(t, reopened.ReserveTVEpisodes(ctx, []int64{a}), "admission blocked")
	require.ErrorContains(t, reopened.ReserveTVEpisodes(ctx, []int64{b}), "admission blocked")
	var rows []struct {
		EpisodeID  int64
		HasCreated bool
	}
	query, err := reopened.(*SQLite).db.Query(`SELECT episode_id, created_at IS NOT NULL FROM tv_episode_admission ORDER BY episode_id`)
	require.NoError(t, err)
	for query.Next() {
		var row struct {
			EpisodeID  int64
			HasCreated bool
		}
		require.NoError(t, query.Scan(&row.EpisodeID, &row.HasCreated))
		rows = append(rows, row)
	}
	require.NoError(t, query.Err())
	require.NoError(t, query.Close())
	require.Equal(t, []struct {
		EpisodeID  int64
		HasCreated bool
	}{{a, true}, {b, true}}, rows)
	// Explicit operator-verified recovery releases exactly the chosen coverage.
	require.NoError(t, reopened.ReleaseTVEpisodes(ctx, []int64{a, b}))
	require.NoError(t, reopened.ReserveTVEpisodes(ctx, []int64{a, b}))
}
