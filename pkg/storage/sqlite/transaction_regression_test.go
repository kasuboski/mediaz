package sqlite

import (
	"database/sql"
	"testing"

	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSQLite_CreateSeasonCommitFailureRollbackRecovery(t *testing.T) {
	for _, mode := range []string{"fresh", "existing-state upsert"} {
		t.Run(mode, func(t *testing.T) {
			ctx := t.Context()
			s := sqlcIntegrationStore(t)
			seriesID := tvTestSeries(t, ctx, s)
			seed := sqlcdb.Season{ID: 100, SeriesID: seriesID, SeasonNumber: 1, Monitored: 1}
			seedID, err := s.CreateSeason(ctx, seed, storage.SeasonStateMissing)
			require.NoError(t, err)
			assert.Equal(t, seed.ID, seedID)
			initial, err := s.GetSeasonByID(ctx, seedID)
			require.NoError(t, err)
			assert.Equal(t, sqlcdb.GetSeasonByIDRow{Season: seed, State: sql.NullString{String: "missing", Valid: true}}, initial)
			require.NoError(t, s.UpdateSeasonState(ctx, seed.ID, storage.SeasonStateDownloading, nil))
			before, err := s.ListSeasons(ctx, storage.SeasonFilter{})
			require.NoError(t, err)
			history := seasonTransitionRows(t, s)
			candidate := sqlcdb.Season{ID: 200, SeriesID: 999999, SeasonNumber: 2}
			if mode == "existing-state upsert" {
				candidate.ID = seed.ID
			}

			// Enable only after setup: the missing parent must fail COMMIT,
			// not the entity INSERT/UPDATE or the initial history INSERT.
			_, err = s.db.ExecContext(ctx, `PRAGMA defer_foreign_keys = ON`)
			require.NoError(t, err)
			id, err := s.CreateSeason(ctx, candidate, storage.SeasonStateDiscovered)
			requireConstraint(t, err, sqlite3.ErrConstraintForeignKey)
			assert.Zero(t, id)
			after, err := s.ListSeasons(ctx, storage.SeasonFilter{})
			require.NoError(t, err)
			assert.Equal(t, before, after)
			assert.Equal(t, history, seasonTransitionRows(t, s))
			// Lists join history, so also rule out a transitionless partial row.
			assert.Equal(t, int64(len(before)), countRows(t, s, "season"))

			candidate.SeriesID = seriesID
			id, err = s.CreateSeason(ctx, candidate, storage.SeasonStateDiscovered)
			require.NoError(t, err)
			assert.Equal(t, candidate.ID, id)
			state := "discovered"
			if mode == "existing-state upsert" {
				state = "downloading"
			}
			got, err := s.GetSeasonByID(ctx, id)
			require.NoError(t, err)
			assert.Equal(t, sqlcdb.GetSeasonByIDRow{Season: candidate, State: sql.NullString{String: state, Valid: true}}, got)
			recovered := seasonTransitionRows(t, s)
			if mode == "existing-state upsert" {
				assert.Equal(t, history, recovered, "upsert must preserve all history, including latest markers and timestamps")
			} else {
				require.Len(t, recovered, len(history)+1)
				assert.Equal(t, history, recovered[:len(history)])
				latest := recovered[len(history)]
				require.Positive(t, latest.ID)
				require.True(t, latest.CreatedAt.Valid)
				require.False(t, latest.CreatedAt.Time.IsZero())
				assert.Equal(t, sqlcdb.SeasonTransition{ID: latest.ID, SeasonID: id, ToState: state, MostRecent: true, SortKey: 1, CreatedAt: latest.CreatedAt, UpdatedAt: latest.CreatedAt}, latest)
			}
		})
	}
}

func TestSQLite_CreateEpisodeCommitFailureRollbackRecovery(t *testing.T) {
	for _, mode := range []string{"fresh", "existing-state upsert"} {
		t.Run(mode, func(t *testing.T) {
			ctx := t.Context()
			s := sqlcIntegrationStore(t)
			seasonID, err := s.CreateSeason(ctx, sqlcdb.Season{SeriesID: tvTestSeries(t, ctx, s), SeasonNumber: 1, Monitored: 1}, storage.SeasonStateMissing)
			require.NoError(t, err)
			seed := sqlcdb.Episode{ID: 100, SeasonID: seasonID, EpisodeNumber: 1, Monitored: 1}
			seedID, err := s.CreateEpisode(ctx, seed, storage.EpisodeStateMissing)
			require.NoError(t, err)
			assert.Equal(t, seed.ID, seedID)
			initial, err := s.GetEpisodeByID(ctx, seedID)
			require.NoError(t, err)
			assert.Equal(t, sqlcdb.GetEpisodeByIDRow{Episode: seed, State: sql.NullString{String: "missing", Valid: true}}, initial)
			require.NoError(t, s.UpdateEpisodeState(ctx, seed.ID, storage.EpisodeStateDownloading, nil))
			before, err := s.ListEpisodes(ctx, storage.EpisodeFilter{})
			require.NoError(t, err)
			history := episodeTransitionRows(t, s)
			candidate := sqlcdb.Episode{ID: 200, SeasonID: 999999, EpisodeNumber: 2}
			if mode == "existing-state upsert" {
				candidate.ID = seed.ID
			}

			_, err = s.db.ExecContext(ctx, `PRAGMA defer_foreign_keys = ON`)
			require.NoError(t, err)
			id, err := s.CreateEpisode(ctx, candidate, storage.EpisodeStateDiscovered)
			requireConstraint(t, err, sqlite3.ErrConstraintForeignKey)
			assert.Zero(t, id)
			after, err := s.ListEpisodes(ctx, storage.EpisodeFilter{})
			require.NoError(t, err)
			assert.Equal(t, before, after)
			assert.Equal(t, history, episodeTransitionRows(t, s))
			assert.Equal(t, int64(len(before)), countRows(t, s, "episode"))

			candidate.SeasonID = seasonID
			id, err = s.CreateEpisode(ctx, candidate, storage.EpisodeStateDiscovered)
			require.NoError(t, err)
			assert.Equal(t, candidate.ID, id)
			state := "discovered"
			if mode == "existing-state upsert" {
				state = "downloading"
			}
			got, err := s.GetEpisodeByID(ctx, id)
			require.NoError(t, err)
			assert.Equal(t, sqlcdb.GetEpisodeByIDRow{Episode: candidate, State: sql.NullString{String: state, Valid: true}}, got)
			recovered := episodeTransitionRows(t, s)
			if mode == "existing-state upsert" {
				assert.Equal(t, history, recovered, "upsert must preserve all history, including latest markers and timestamps")
			} else {
				require.Len(t, recovered, len(history)+1)
				assert.Equal(t, history, recovered[:len(history)])
				latest := recovered[len(history)]
				require.Positive(t, latest.ID)
				require.True(t, latest.CreatedAt.Valid)
				require.False(t, latest.CreatedAt.Time.IsZero())
				assert.Equal(t, sqlcdb.EpisodeTransition{ID: latest.ID, EpisodeID: id, ToState: state, MostRecent: true, SortKey: 1, CreatedAt: latest.CreatedAt, UpdatedAt: latest.CreatedAt}, latest)
			}
		})
	}
}

func TestSQLite_CreateSeriesTransitionFailureRollbackRecovery(t *testing.T) {
	ctx := t.Context()
	s := sqlcIntegrationStore(t)
	siblingID := tvTestSeries(t, ctx, s)
	before, err := s.ListSeries(ctx, storage.SeriesFilter{})
	require.NoError(t, err)
	history := seriesTransitionRows(t, s, siblingID)
	series := sqlcdb.Series{Path: sql.NullString{String: "/tv/rollback", Valid: true}, Monitored: 1, QualityProfileID: 1, MonitorNewSeasons: 1}
	const trigger = "abort_create_series_transition"
	installAbortTrigger(t, s, trigger, "series_transition", "INSERT")
	t.Cleanup(func() { dropTrigger(t, s, trigger) })

	id, err := s.CreateSeries(ctx, series, storage.SeriesStateMissing)
	require.ErrorContains(t, err, "injected failure: INSERT series_transition")
	requireConstraint(t, err, sqlite3.ErrConstraintTrigger)
	assert.Zero(t, id)
	after, err := s.ListSeries(ctx, storage.SeriesFilter{})
	require.NoError(t, err)
	assert.Equal(t, before, after)
	assert.Equal(t, history, seriesTransitionRows(t, s, siblingID))
	assert.Equal(t, int64(len(before)), countRows(t, s, "series"))
	assert.Equal(t, int64(len(history)), countRows(t, s, "series_transition"))
	_, err = s.GetSeriesByPath(ctx, series.Path.String)
	assert.ErrorIs(t, err, storage.ErrNotFound)

	dropTrigger(t, s, trigger)
	id, err = s.CreateSeries(ctx, series, storage.SeriesStateMissing)
	require.NoError(t, err)
	require.Positive(t, id)
	got, err := s.GetSeriesByID(ctx, id)
	require.NoError(t, err)
	series.ID = id
	require.True(t, got.Series.Added.Valid)
	require.False(t, got.Series.Added.Time.IsZero())
	series.Added = got.Series.Added
	assert.Equal(t, sqlcdb.GetSeriesRow{Series: series, State: "missing"}, got)
	assert.Equal(t, history, seriesTransitionRows(t, s, siblingID))
	recovered := seriesTransitionRows(t, s, id)
	require.Len(t, recovered, 1)
	latest := recovered[0]
	require.Positive(t, latest.ID)
	require.True(t, latest.CreatedAt.Valid)
	require.False(t, latest.CreatedAt.Time.IsZero())
	assert.Equal(t, sqlcdb.SeriesTransition{ID: latest.ID, SeriesID: id, ToState: "missing", MostRecent: true, SortKey: 1, CreatedAt: latest.CreatedAt, UpdatedAt: latest.CreatedAt}, latest)
}
