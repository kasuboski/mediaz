package sqlite

import (
	"database/sql"
	"testing"

	"github.com/kasuboski/mediaz/pkg/ptr"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Read every native history column, including timestamps and latest markers.
// Reading the entire table also detects orphan inserts and changes to siblings.
func seasonTransitionRows(t *testing.T, s *SQLite) []sqlcdb.SeasonTransition {
	t.Helper()
	rows, err := s.db.Query(`SELECT id, season_id, to_state, from_state, most_recent, sort_key,
		download_client_id, download_id, is_entire_season_download, created_at, updated_at
		FROM season_transition ORDER BY id`)
	require.NoError(t, err)
	defer rows.Close()
	var out []sqlcdb.SeasonTransition
	for rows.Next() {
		var r sqlcdb.SeasonTransition
		require.NoError(t, rows.Scan(&r.ID, &r.SeasonID, &r.ToState, &r.FromState, &r.MostRecent, &r.SortKey,
			&r.DownloadClientID, &r.DownloadID, &r.IsEntireSeasonDownload, &r.CreatedAt, &r.UpdatedAt))
		out = append(out, r)
	}
	require.NoError(t, rows.Err())
	return out
}

func episodeTransitionRows(t *testing.T, s *SQLite) []sqlcdb.EpisodeTransition {
	t.Helper()
	rows, err := s.db.Query(`SELECT id, episode_id, to_state, from_state, most_recent, sort_key,
		download_client_id, download_id, is_entire_season_download, created_at, updated_at
		FROM episode_transition ORDER BY id`)
	require.NoError(t, err)
	defer rows.Close()
	var out []sqlcdb.EpisodeTransition
	for rows.Next() {
		var r sqlcdb.EpisodeTransition
		require.NoError(t, rows.Scan(&r.ID, &r.EpisodeID, &r.ToState, &r.FromState, &r.MostRecent, &r.SortKey,
			&r.DownloadClientID, &r.DownloadID, &r.IsEntireSeasonDownload, &r.CreatedAt, &r.UpdatedAt))
		out = append(out, r)
	}
	require.NoError(t, rows.Err())
	return out
}

func TestSQLite_CreateSeasonTransitionFailureRollbackRecovery(t *testing.T) {
	ctx := t.Context()
	s := sqlcIntegrationStore(t)
	seriesID := tvTestSeries(t, ctx, s)
	_, err := s.CreateSeason(ctx, sqlcdb.Season{SeriesID: seriesID, SeasonNumber: 1, Monitored: 1}, storage.SeasonStateMissing)
	require.NoError(t, err)
	before, err := s.ListSeasons(ctx, storage.SeasonFilter{})
	require.NoError(t, err)
	history := seasonTransitionRows(t, s)
	season := sqlcdb.Season{ID: 100, SeriesID: seriesID, SeasonNumber: 2, Monitored: 1}
	const trigger = "abort_create_season_transition"
	installAbortTrigger(t, s, trigger, "season_transition", "INSERT")
	t.Cleanup(func() { dropTrigger(t, s, trigger) })

	id, err := s.CreateSeason(ctx, season, storage.SeasonStateMissing)
	require.ErrorContains(t, err, "injected failure: INSERT season_transition")
	assert.Zero(t, id)
	_, err = s.GetSeasonByID(ctx, season.ID)
	assert.ErrorIs(t, err, storage.ErrNotFound)
	after, err := s.ListSeasons(ctx, storage.SeasonFilter{})
	require.NoError(t, err)
	assert.Equal(t, before, after)
	assert.Equal(t, history, seasonTransitionRows(t, s))
	// The joined API can hide a transitionless entity; inspect the base table too.
	var partial int
	require.NoError(t, s.db.QueryRow(`SELECT COUNT(*) FROM season WHERE id = ?`, season.ID).Scan(&partial))
	assert.Zero(t, partial)

	dropTrigger(t, s, trigger)
	id, err = s.CreateSeason(ctx, season, storage.SeasonStateMissing)
	require.NoError(t, err)
	assert.Equal(t, season.ID, id)
	got, err := s.GetSeasonByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, sqlcdb.GetSeasonByIDRow{Season: season, State: sql.NullString{String: "missing", Valid: true}}, got)
	recovered := seasonTransitionRows(t, s)
	require.Len(t, recovered, len(history)+1)
	assert.Equal(t, history, recovered[:len(history)])
	latest := recovered[len(history)]
	require.Positive(t, latest.ID)
	require.True(t, latest.CreatedAt.Valid)
	require.False(t, latest.CreatedAt.Time.IsZero())
	assert.Equal(t, sqlcdb.SeasonTransition{ID: latest.ID, SeasonID: id, ToState: "missing", MostRecent: true, SortKey: 1, CreatedAt: latest.CreatedAt, UpdatedAt: latest.CreatedAt}, latest)
}

func TestSQLite_CreateEpisodeTransitionFailureRollbackRecovery(t *testing.T) {
	ctx := t.Context()
	s := sqlcIntegrationStore(t)
	seasonID, err := s.CreateSeason(ctx, sqlcdb.Season{SeriesID: tvTestSeries(t, ctx, s), SeasonNumber: 1, Monitored: 1}, storage.SeasonStateMissing)
	require.NoError(t, err)
	_, err = s.CreateEpisode(ctx, sqlcdb.Episode{SeasonID: seasonID, EpisodeNumber: 1, Monitored: 1}, storage.EpisodeStateMissing)
	require.NoError(t, err)
	before, err := s.ListEpisodes(ctx, storage.EpisodeFilter{})
	require.NoError(t, err)
	history := episodeTransitionRows(t, s)
	episode := sqlcdb.Episode{ID: 100, SeasonID: seasonID, EpisodeNumber: 2, Monitored: 1}
	const trigger = "abort_create_episode_transition"
	installAbortTrigger(t, s, trigger, "episode_transition", "INSERT")
	t.Cleanup(func() { dropTrigger(t, s, trigger) })

	id, err := s.CreateEpisode(ctx, episode, storage.EpisodeStateMissing)
	require.ErrorContains(t, err, "injected failure: INSERT episode_transition")
	assert.Zero(t, id)
	_, err = s.GetEpisodeByID(ctx, episode.ID)
	assert.ErrorIs(t, err, storage.ErrNotFound)
	after, err := s.ListEpisodes(ctx, storage.EpisodeFilter{})
	require.NoError(t, err)
	assert.Equal(t, before, after)
	assert.Equal(t, history, episodeTransitionRows(t, s))
	var partial int
	require.NoError(t, s.db.QueryRow(`SELECT COUNT(*) FROM episode WHERE id = ?`, episode.ID).Scan(&partial))
	assert.Zero(t, partial)

	dropTrigger(t, s, trigger)
	id, err = s.CreateEpisode(ctx, episode, storage.EpisodeStateMissing)
	require.NoError(t, err)
	assert.Equal(t, episode.ID, id)
	got, err := s.GetEpisodeByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, sqlcdb.GetEpisodeByIDRow{Episode: episode, State: sql.NullString{String: "missing", Valid: true}}, got)
	recovered := episodeTransitionRows(t, s)
	require.Len(t, recovered, len(history)+1)
	assert.Equal(t, history, recovered[:len(history)])
	latest := recovered[len(history)]
	require.Positive(t, latest.ID)
	require.True(t, latest.CreatedAt.Valid)
	require.False(t, latest.CreatedAt.Time.IsZero())
	assert.Equal(t, sqlcdb.EpisodeTransition{ID: latest.ID, EpisodeID: id, ToState: "missing", MostRecent: true, SortKey: 1, CreatedAt: latest.CreatedAt, UpdatedAt: latest.CreatedAt}, latest)
}

func TestSQLite_UpdateSeasonTransitionFailureRollbackRecovery(t *testing.T) {
	ctx := t.Context()
	s := sqlcIntegrationStore(t)
	season := sqlcdb.Season{SeriesID: tvTestSeries(t, ctx, s), SeasonNumber: 3, Monitored: 1}
	id, err := s.CreateSeason(ctx, season, storage.SeasonStateMissing)
	require.NoError(t, err)
	season.ID = id
	clientID, err := s.CreateDownloadClient(ctx, sqlcdb.DownloadClient{Type: "torrent", Implementation: "transmission", Host: "localhost", Scheme: "http"})
	require.NoError(t, err)
	require.NoError(t, s.UpdateSeasonState(ctx, id, storage.SeasonStateDownloading, &storage.TransitionStateMetadata{DownloadID: ptr.To("season-bundle"), DownloadClientID: &clientID, IsEntireSeasonDownload: ptr.To(true)}))
	before, err := s.GetSeasonByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, sqlcdb.GetSeasonByIDRow{Season: season, State: sql.NullString{String: "downloading", Valid: true}, DownloadID: sql.NullString{String: "season-bundle", Valid: true}, DownloadClientID: sql.NullInt64{Int64: clientID, Valid: true}, IsEntireSeasonDownload: sql.NullBool{Bool: true, Valid: true}}, before)
	history := seasonTransitionRows(t, s)
	require.Len(t, history, 2)
	assert.False(t, history[0].MostRecent)
	assert.True(t, history[1].MostRecent)
	const trigger = "abort_update_season_transition"
	installAbortTrigger(t, s, trigger, "season_transition", "INSERT")
	t.Cleanup(func() { dropTrigger(t, s, trigger) })

	err = s.UpdateSeasonState(ctx, id, storage.SeasonStateCompleted, nil)
	require.ErrorContains(t, err, "injected failure: INSERT season_transition")
	after, err := s.GetSeasonByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, before, after)
	assert.Equal(t, history, seasonTransitionRows(t, s))

	dropTrigger(t, s, trigger)
	require.NoError(t, s.UpdateSeasonState(ctx, id, storage.SeasonStateCompleted, nil))
	after, err = s.GetSeasonByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, sqlcdb.GetSeasonByIDRow{Season: season, State: sql.NullString{String: "completed", Valid: true}}, after)
	recovered := seasonTransitionRows(t, s)
	require.Len(t, recovered, 3)
	assert.Equal(t, history[0], recovered[0])
	// Demotion changes only the latest marker and its update timestamp.
	expectedPrevious := history[1]
	expectedPrevious.MostRecent = false
	expectedPrevious.UpdatedAt = recovered[1].UpdatedAt
	require.True(t, expectedPrevious.UpdatedAt.Valid)
	assert.False(t, expectedPrevious.UpdatedAt.Time.Before(history[1].UpdatedAt.Time))
	assert.Equal(t, expectedPrevious, recovered[1])
	latest := recovered[2]
	assert.Greater(t, latest.ID, history[1].ID)
	require.True(t, latest.CreatedAt.Valid)
	require.False(t, latest.CreatedAt.Time.IsZero())
	assert.Equal(t, sqlcdb.SeasonTransition{ID: latest.ID, SeasonID: id, FromState: sql.NullString{String: "downloading", Valid: true}, ToState: "completed", MostRecent: true, SortKey: 3, CreatedAt: latest.CreatedAt, UpdatedAt: latest.CreatedAt}, latest)
}

func TestSQLite_UpdateEpisodeTransitionFailureRollbackRecovery(t *testing.T) {
	ctx := t.Context()
	s := sqlcIntegrationStore(t)
	seasonID, err := s.CreateSeason(ctx, sqlcdb.Season{SeriesID: tvTestSeries(t, ctx, s), SeasonNumber: 1, Monitored: 1}, storage.SeasonStateMissing)
	require.NoError(t, err)
	episode := sqlcdb.Episode{SeasonID: seasonID, EpisodeNumber: 3, Monitored: 1}
	id, err := s.CreateEpisode(ctx, episode, storage.EpisodeStateMissing)
	require.NoError(t, err)
	episode.ID = id
	clientID, err := s.CreateDownloadClient(ctx, sqlcdb.DownloadClient{Type: "torrent", Implementation: "transmission", Host: "localhost", Scheme: "http"})
	require.NoError(t, err)
	require.NoError(t, s.UpdateEpisodeState(ctx, id, storage.EpisodeStateDownloading, &storage.TransitionStateMetadata{DownloadID: ptr.To("episode-bundle"), DownloadClientID: &clientID, IsEntireSeasonDownload: ptr.To(true)}))
	before, err := s.GetEpisodeByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, sqlcdb.GetEpisodeByIDRow{Episode: episode, State: sql.NullString{String: "downloading", Valid: true}, DownloadID: sql.NullString{String: "episode-bundle", Valid: true}, DownloadClientID: sql.NullInt64{Int64: clientID, Valid: true}, IsEntireSeasonDownload: sql.NullBool{Bool: true, Valid: true}}, before)
	history := episodeTransitionRows(t, s)
	require.Len(t, history, 2)
	assert.False(t, history[0].MostRecent)
	assert.True(t, history[1].MostRecent)
	const trigger = "abort_update_episode_transition"
	installAbortTrigger(t, s, trigger, "episode_transition", "INSERT")
	t.Cleanup(func() { dropTrigger(t, s, trigger) })

	err = s.UpdateEpisodeState(ctx, id, storage.EpisodeStateDownloaded, nil)
	require.ErrorContains(t, err, "injected failure: INSERT episode_transition")
	after, err := s.GetEpisodeByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, before, after)
	assert.Equal(t, history, episodeTransitionRows(t, s))

	dropTrigger(t, s, trigger)
	require.NoError(t, s.UpdateEpisodeState(ctx, id, storage.EpisodeStateDownloaded, nil))
	after, err = s.GetEpisodeByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, sqlcdb.GetEpisodeByIDRow{Episode: episode, State: sql.NullString{String: "downloaded", Valid: true}}, after)
	recovered := episodeTransitionRows(t, s)
	require.Len(t, recovered, 3)
	assert.Equal(t, history[0], recovered[0])
	expectedPrevious := history[1]
	expectedPrevious.MostRecent = false
	expectedPrevious.UpdatedAt = recovered[1].UpdatedAt
	require.True(t, expectedPrevious.UpdatedAt.Valid)
	assert.False(t, expectedPrevious.UpdatedAt.Time.Before(history[1].UpdatedAt.Time))
	assert.Equal(t, expectedPrevious, recovered[1])
	latest := recovered[2]
	assert.Greater(t, latest.ID, history[1].ID)
	require.True(t, latest.CreatedAt.Valid)
	require.False(t, latest.CreatedAt.Time.IsZero())
	assert.Equal(t, sqlcdb.EpisodeTransition{ID: latest.ID, EpisodeID: id, FromState: sql.NullString{String: "downloading", Valid: true}, ToState: "downloaded", MostRecent: true, SortKey: 3, CreatedAt: latest.CreatedAt, UpdatedAt: latest.CreatedAt}, latest)
}
