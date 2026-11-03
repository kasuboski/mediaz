package sqlite

import (
	"context"
	"database/sql"
	"testing"

	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newSelectionTestStore boots an in-memory SQLite store seeded with one
// series, one season and one episode and returns the store plus their IDs.
func newSelectionTestStore(t *testing.T) (storage.Storage, int64, int64, int64) {
	t.Helper()
	ctx := context.Background()

	store, err := New(ctx, inMemoryPath)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.(*SQLite).db.Close()) })

	seriesID, err := store.CreateSeries(ctx, sqlcdb.Series{
		QualityProfileID:  1,
		Monitored:         1,
		MonitorNewSeasons: 0,
	}, storage.SeriesStateDiscovered)
	require.NoError(t, err)

	seasonID, err := store.CreateSeason(ctx, sqlcdb.Season{
		SeriesID:     seriesID,
		SeasonNumber: 1,
		Monitored:    1,
	}, storage.SeasonStateDiscovered)
	require.NoError(t, err)

	episodeID, err := store.CreateEpisode(ctx, sqlcdb.Episode{
		SeasonID:      seasonID,
		EpisodeNumber: 1,
		Monitored:     1,
	}, storage.EpisodeStateDiscovered)
	require.NoError(t, err)

	return store, seriesID, seasonID, episodeID
}

func selectionOverride(t *testing.T, store storage.Storage, seasonID, episodeID int64) (season sql.NullInt64, episode sql.NullInt64) {
	t.Helper()
	ctx := context.Background()
	s, err := store.GetSeasonByID(ctx, seasonID)
	require.NoError(t, err)
	e, err := store.GetEpisodeByID(ctx, episodeID)
	require.NoError(t, err)
	return s.Season.QualityProfileID, e.Episode.QualityProfileID
}

func TestSeasonEpisodeQualityOverride_DefaultsNullAndSetClear(t *testing.T) {
	store, _, seasonID, episodeID := newSelectionTestStore(t)
	ctx := context.Background()

	// Fresh children inherit (NULL).
	seasonOverride, episodeOverride := selectionOverride(t, store, seasonID, episodeID)
	assert.False(t, seasonOverride.Valid)
	assert.False(t, episodeOverride.Valid)

	// Set the season override to an existing series profile.
	require.NoError(t, store.UpdateSeasonQualityProfile(ctx, seasonID, sql.NullInt64{Int64: 2, Valid: true}))
	// Set the episode override.
	require.NoError(t, store.UpdateEpisodeQualityProfile(ctx, episodeID, sql.NullInt64{Int64: 3, Valid: true}))

	seasonOverride, episodeOverride = selectionOverride(t, store, seasonID, episodeID)
	require.True(t, seasonOverride.Valid)
	assert.EqualValues(t, 2, seasonOverride.Int64)
	require.True(t, episodeOverride.Valid)
	assert.EqualValues(t, 3, episodeOverride.Int64)

	// Clearing (NULL) restores inheritance.
	require.NoError(t, store.UpdateSeasonQualityProfile(ctx, seasonID, sql.NullInt64{}))
	require.NoError(t, store.UpdateEpisodeQualityProfile(ctx, episodeID, sql.NullInt64{}))
	seasonOverride, episodeOverride = selectionOverride(t, store, seasonID, episodeID)
	assert.False(t, seasonOverride.Valid)
	assert.False(t, episodeOverride.Valid)
}

func TestSeasonEpisodeQualityOverride_RefreshUpsertPreservesOverride(t *testing.T) {
	store, seriesID, seasonID, episodeID := newSelectionTestStore(t)
	ctx := context.Background()

	require.NoError(t, store.UpdateSeasonQualityProfile(ctx, seasonID, sql.NullInt64{Int64: 2, Valid: true}))
	require.NoError(t, store.UpdateEpisodeQualityProfile(ctx, episodeID, sql.NullInt64{Int64: 3, Valid: true}))

	// Metadata refresh upserts by id with a NULL override in the struct; the
	// stored overrides must survive.
	_, err := store.CreateSeason(ctx, sqlcdb.Season{
		ID:           seasonID,
		SeriesID:     seriesID,
		SeasonNumber: 1,
		Monitored:    0,
	}, storage.SeasonStateDiscovered)
	require.NoError(t, err)
	_, err = store.CreateEpisode(ctx, sqlcdb.Episode{
		ID:            episodeID,
		SeasonID:      seasonID,
		EpisodeNumber: 1,
		Monitored:     1,
	}, storage.EpisodeStateDiscovered)
	require.NoError(t, err)

	season, err := store.GetSeasonByID(ctx, seasonID)
	require.NoError(t, err)
	require.Equal(t, sqlcdb.Season{ID: seasonID, SeriesID: seriesID, SeasonNumber: 1, Monitored: 1, QualityProfileID: sql.NullInt64{Int64: 2, Valid: true}}, season.Season)
	seasonOverride, episodeOverride := selectionOverride(t, store, seasonID, episodeID)
	require.True(t, seasonOverride.Valid, "season override must survive refresh upsert")
	assert.EqualValues(t, 2, seasonOverride.Int64)
	require.True(t, episodeOverride.Valid, "episode override must survive refresh upsert")
	assert.EqualValues(t, 3, episodeOverride.Int64)

	// A stale refresh carrying a different policy must not replace choices.
	_, err = store.CreateEpisode(ctx, sqlcdb.Episode{
		ID:               episodeID,
		SeasonID:         seasonID,
		EpisodeNumber:    1,
		Monitored:        0,
		QualityProfileID: sql.NullInt64{Int64: 2, Valid: true},
	}, storage.EpisodeStateDiscovered)
	require.NoError(t, err)
	_, episodeOverride = selectionOverride(t, store, seasonID, episodeID)
	require.True(t, episodeOverride.Valid)
	assert.EqualValues(t, 3, episodeOverride.Int64)
	episode, err := store.GetEpisodeByID(ctx, episodeID)
	require.NoError(t, err)
	require.Equal(t, sqlcdb.Episode{ID: episodeID, SeasonID: seasonID, EpisodeNumber: 1, Monitored: 1, QualityProfileID: sql.NullInt64{Int64: 3, Valid: true}}, episode.Episode)
}

func TestApplySeriesSelectionPatch_AppliesAllFieldsAtomically(t *testing.T) {
	store, seriesID, seasonID, episodeID := newSelectionTestStore(t)
	ctx := context.Background()

	// Seed a season override that the patch clears.
	require.NoError(t, store.UpdateSeasonQualityProfile(ctx, seasonID, sql.NullInt64{Int64: 2, Valid: true}))
	clearOverride := int64(0)
	newSeriesProfile := int64(3)

	require.NoError(t, store.ApplySeriesSelectionPatch(ctx, seriesID, storage.SeriesSelectionPatch{
		SeriesMonitored:   boolPtr(false),
		MonitorNewSeasons: boolPtr(true),
		QualityProfileID:  &newSeriesProfile,
		Seasons: []storage.SeasonSelectionPatch{{
			ID:        seasonID,
			Monitored: boolPtr(false),
			// Clear the pre-existing override back to inheritance.
			QualityProfileID: &clearOverride,
		}},
		Episodes: []storage.EpisodeSelectionPatch{{
			ID:               episodeID,
			Monitored:        boolPtr(false),
			QualityProfileID: &newSeriesProfile,
		}},
	}))

	series, err := store.GetSeriesByID(ctx, seriesID)
	require.NoError(t, err)
	assert.EqualValues(t, 0, series.Series.Monitored)
	assert.EqualValues(t, 1, series.Series.MonitorNewSeasons)
	assert.EqualValues(t, 3, series.Series.QualityProfileID)

	season, err := store.GetSeasonByID(ctx, seasonID)
	require.NoError(t, err)
	assert.EqualValues(t, 0, season.Season.Monitored)
	assert.False(t, season.Season.QualityProfileID.Valid, "cleared override must be NULL")

	episode, err := store.GetEpisodeByID(ctx, episodeID)
	require.NoError(t, err)
	assert.EqualValues(t, 0, episode.Episode.Monitored)
	require.True(t, episode.Episode.QualityProfileID.Valid)
	assert.EqualValues(t, 3, episode.Episode.QualityProfileID.Int64)
}

func TestApplySeriesSelectionPatch_SparseFieldsUnchanged(t *testing.T) {
	store, seriesID, seasonID, episodeID := newSelectionTestStore(t)
	ctx := context.Background()

	// Empty patch is a no-op that still verifies series existence.
	require.NoError(t, store.ApplySeriesSelectionPatch(ctx, seriesID, storage.SeriesSelectionPatch{}))

	// Only one field touched: everything else stays.
	require.NoError(t, store.ApplySeriesSelectionPatch(ctx, seriesID, storage.SeriesSelectionPatch{
		Seasons: []storage.SeasonSelectionPatch{{ID: seasonID, Monitored: boolPtr(false)}},
	}))

	season, err := store.GetSeasonByID(ctx, seasonID)
	require.NoError(t, err)
	assert.EqualValues(t, 0, season.Season.Monitored)
	assert.False(t, season.Season.QualityProfileID.Valid)

	series, err := store.GetSeriesByID(ctx, seriesID)
	require.NoError(t, err)
	assert.EqualValues(t, 1, series.Series.Monitored, "series monitored must be untouched")
	assert.EqualValues(t, 1, series.Series.QualityProfileID)
	episode, err := store.GetEpisodeByID(ctx, episodeID)
	require.NoError(t, err)
	assert.EqualValues(t, 1, episode.Episode.Monitored, "sibling episode must be untouched")
}

func TestApplySeriesSelectionPatch_RollbackOnForeignChild(t *testing.T) {
	store, seriesID, seasonID, episodeID := newSelectionTestStore(t)
	ctx := context.Background()

	// A second series owns a season the patch must not be able to touch.
	otherSeriesID, err := store.CreateSeries(ctx, sqlcdb.Series{
		QualityProfileID: 1,
		Monitored:        1,
	}, storage.SeriesStateDiscovered)
	require.NoError(t, err)
	foreignSeasonID, err := store.CreateSeason(ctx, sqlcdb.Season{
		SeriesID:     otherSeriesID,
		SeasonNumber: 1,
		Monitored:    1,
	}, storage.SeasonStateDiscovered)
	require.NoError(t, err)

	require.ErrorIs(t, store.ApplySeriesSelectionPatch(ctx, seriesID, storage.SeriesSelectionPatch{
		Seasons:  []storage.SeasonSelectionPatch{{ID: foreignSeasonID, Monitored: boolPtr(false)}},
		Episodes: []storage.EpisodeSelectionPatch{{ID: episodeID, Monitored: boolPtr(false)}},
	}), storage.ErrNotOwned)

	// Nothing may have been written.
	season, err := store.GetSeasonByID(ctx, foreignSeasonID)
	require.NoError(t, err)
	assert.EqualValues(t, 1, season.Season.Monitored, "foreign season must stay untouched")
	season, err = store.GetSeasonByID(ctx, seasonID)
	require.NoError(t, err)
	assert.EqualValues(t, 1, season.Season.Monitored, "valid entry must roll back with the invalid one")
	episode, err := store.GetEpisodeByID(ctx, episodeID)
	require.NoError(t, err)
	assert.EqualValues(t, 1, episode.Episode.Monitored)
}

func TestApplySeriesSelectionPatch_RollbackOnUnknownProfile(t *testing.T) {
	store, seriesID, seasonID, _ := newSelectionTestStore(t)
	ctx := context.Background()

	unknownProfile := int64(999)
	require.ErrorIs(t, store.ApplySeriesSelectionPatch(ctx, seriesID, storage.SeriesSelectionPatch{
		Seasons: []storage.SeasonSelectionPatch{{
			ID:               seasonID,
			Monitored:        boolPtr(false),
			QualityProfileID: &unknownProfile,
		}},
	}), storage.ErrNotFound)

	season, err := store.GetSeasonByID(ctx, seasonID)
	require.NoError(t, err)
	assert.EqualValues(t, 1, season.Season.Monitored, "failed patch must roll back")
	assert.False(t, season.Season.QualityProfileID.Valid)
}

func TestApplySeriesSelectionPatch_ValidationErrors(t *testing.T) {
	store, seriesID, seasonID, episodeID := newSelectionTestStore(t)
	ctx := context.Background()

	// Duplicate season ids in one patch.
	dup := storage.SeriesSelectionPatch{Seasons: []storage.SeasonSelectionPatch{
		{ID: seasonID, Monitored: boolPtr(false)},
		{ID: seasonID, Monitored: boolPtr(true)},
	}}
	require.ErrorContains(t, store.ApplySeriesSelectionPatch(ctx, seriesID, dup), "duplicate season id")

	// Duplicate episode ids in one patch.
	dup = storage.SeriesSelectionPatch{Episodes: []storage.EpisodeSelectionPatch{
		{ID: episodeID, Monitored: boolPtr(false)},
		{ID: episodeID},
	}}
	require.ErrorContains(t, store.ApplySeriesSelectionPatch(ctx, seriesID, dup), "duplicate episode id")

	// Unknown series.
	require.ErrorIs(t, store.ApplySeriesSelectionPatch(ctx, 12345, storage.SeriesSelectionPatch{}), storage.ErrNotFound)

	// Unknown episode id.
	require.ErrorIs(t, store.ApplySeriesSelectionPatch(ctx, seriesID, storage.SeriesSelectionPatch{
		Episodes: []storage.EpisodeSelectionPatch{{ID: 12345}},
	}), storage.ErrNotFound)

	// Non-positive child id.
	require.ErrorContains(t, store.ApplySeriesSelectionPatch(ctx, seriesID, storage.SeriesSelectionPatch{
		Seasons: []storage.SeasonSelectionPatch{{ID: 0}},
	}), "invalid season id")
}

func boolPtr(b bool) *bool {
	return &b
}
