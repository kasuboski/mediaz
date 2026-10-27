package sqlite

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func requireConstraint(t *testing.T, err error, code sqlite3.ErrNoExtended) {
	t.Helper()
	var constraint sqlite3.Error
	require.True(t, errors.As(err, &constraint), "expected SQLite constraint, got %v", err)
	require.Equal(t, code, constraint.ExtendedCode)
}

func TestJobTransitionConstraintsApplyToDirectGeneratedQueries(t *testing.T) {
	ctx := t.Context()
	store := initSqlite(t, ctx)
	q := store.(*SQLite).Queries
	id, err := store.CreateJob(ctx, sqlcdb.Job{Type: "invariant-one"}, storage.JobStatePending)
	require.NoError(t, err)
	before, err := store.GetJob(ctx, id)
	require.NoError(t, err)
	// Use running, not pending: this tests the per-job latest index independently
	// of the unique pending-per-type index.
	err = q.CreateJobTransition(ctx, sqlcdb.CreateJobTransitionParams{JobID: id, Type: "invariant-one", ToState: "running", MostRecent: true, SortKey: 2})
	requireConstraint(t, err, sqlite3.ErrConstraintUnique)
	require.ErrorContains(t, err, "job_transition.job_id")
	err = q.CreateJobTransition(ctx, sqlcdb.CreateJobTransitionParams{JobID: id, Type: "invariant-one", ToState: "running", MostRecent: false, SortKey: 1})
	requireConstraint(t, err, sqlite3.ErrConstraintUnique)
	require.ErrorContains(t, err, "job_transition.sort_key")
	// Historical rows remain legal, and sort keys are scoped to the parent.
	require.NoError(t, q.CreateJobTransition(ctx, sqlcdb.CreateJobTransitionParams{JobID: id, Type: "invariant-one", ToState: "running", MostRecent: false, SortKey: 2}))
	other, err := store.CreateJob(ctx, sqlcdb.Job{Type: "invariant-two"}, storage.JobStatePending)
	require.NoError(t, err)
	require.NoError(t, q.CreateJobTransition(ctx, sqlcdb.CreateJobTransitionParams{JobID: other, Type: "invariant-two", ToState: "running", MostRecent: false, SortKey: 2}))
	after, err := store.GetJob(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, before, after, "rejected inserts and historical rows must not change the latest state")
	history, err := store.GetEntityTransitionsJob(ctx, id)
	require.NoError(t, err)
	require.Len(t, history, 2)
}

func TestReferencedQualityProfilesCannotBeDeleted(t *testing.T) {
	ctx := t.Context()
	store := initSqlite(t, ctx)
	profile := sqlcdb.QualityProfile{Name: "in-use", UpgradeAllowed: true}
	profileID, err := store.CreateQualityProfile(ctx, profile)
	require.NoError(t, err)
	profile.ID = profileID
	movie, err := store.CreateMovie(ctx, sqlcdb.Movie{QualityProfileID: profileID}, storage.MovieStateMissing)
	require.NoError(t, err)
	series, err := store.CreateSeries(ctx, sqlcdb.Series{QualityProfileID: profileID}, storage.SeriesStateMissing)
	require.NoError(t, err)
	requireConstraint(t, store.DeleteQualityProfile(ctx, profileID), sqlite3.ErrConstraintTrigger)
	got, err := store.GetQualityProfile(ctx, profileID)
	require.NoError(t, err)
	assert.Equal(t, profile, got)
	require.NoError(t, store.DeleteMovie(ctx, movie))
	requireConstraint(t, store.DeleteQualityProfile(ctx, profileID), sqlite3.ErrConstraintTrigger)
	require.NoError(t, store.DeleteSeries(ctx, series))
	require.NoError(t, store.DeleteQualityProfile(ctx, profileID))
	_, err = store.GetQualityProfile(ctx, profileID)
	require.ErrorIs(t, err, storage.ErrNotFound)
}

func TestOptionalMediaReferencesSetNullAndMetadataAncestryRestricts(t *testing.T) {
	ctx := t.Context()
	store := initSqlite(t, ctx)
	movieMetadata, err := store.CreateMovieMetadata(ctx, sqlcdb.MovieMetadatum{TmdbID: 111, Title: "Movie"})
	require.NoError(t, err)
	movieFile, err := store.CreateMovieFile(ctx, sqlcdb.MovieFile{Quality: "HD", Languages: "[]"})
	require.NoError(t, err)
	movie, err := store.CreateMovie(ctx, sqlcdb.Movie{QualityProfileID: 1, MovieMetadataID: sql.NullInt64{Int64: movieMetadata, Valid: true}, MovieFileID: sql.NullInt64{Int64: movieFile, Valid: true}}, storage.MovieStateMissing)
	require.NoError(t, err)
	expectedMovie, err := store.GetMovie(ctx, movie)
	require.NoError(t, err)
	require.NoError(t, store.DeleteMovieFile(ctx, movieFile))
	expectedMovie.Movie.MovieFileID = sql.NullInt64{}
	gotMovie, err := store.GetMovie(ctx, movie)
	require.NoError(t, err)
	assert.Equal(t, expectedMovie, gotMovie)
	require.NoError(t, store.DeleteMovieMetadata(ctx, movieMetadata))
	expectedMovie.Movie.MovieMetadataID = sql.NullInt64{}
	gotMovie, err = store.GetMovie(ctx, movie)
	require.NoError(t, err)
	assert.Equal(t, expectedMovie, gotMovie)

	seriesMetadata, err := store.CreateSeriesMetadata(ctx, sqlcdb.SeriesMetadatum{TmdbID: 222, Title: "Series"})
	require.NoError(t, err)
	seasonMetadata, err := store.CreateSeasonMetadata(ctx, sqlcdb.SeasonMetadatum{SeriesMetadataID: seriesMetadata, TmdbID: 333, Number: 1, Title: "Season"})
	require.NoError(t, err)
	episodeMetadata, err := store.CreateEpisodeMetadata(ctx, sqlcdb.EpisodeMetadatum{SeasonMetadataID: seasonMetadata, TmdbID: 444, Number: 1, Title: "Episode"})
	require.NoError(t, err)
	episodeFile, err := store.CreateEpisodeFile(ctx, sqlcdb.EpisodeFile{Quality: "HD"})
	require.NoError(t, err)
	series, err := store.CreateSeries(ctx, sqlcdb.Series{QualityProfileID: 1, SeriesMetadataID: sql.NullInt64{Int64: seriesMetadata, Valid: true}}, storage.SeriesStateMissing)
	require.NoError(t, err)
	season, err := store.CreateSeason(ctx, sqlcdb.Season{SeriesID: series, SeasonNumber: 1, SeasonMetadataID: sql.NullInt64{Int64: seasonMetadata, Valid: true}}, storage.SeasonStateMissing)
	require.NoError(t, err)
	episode, err := store.CreateEpisode(ctx, sqlcdb.Episode{SeasonID: season, EpisodeNumber: 1, EpisodeMetadataID: sql.NullInt64{Int64: episodeMetadata, Valid: true}, EpisodeFileID: sql.NullInt64{Int64: episodeFile, Valid: true}}, storage.EpisodeStateMissing)
	require.NoError(t, err)
	expectedSeries, err := store.GetSeriesByID(ctx, series)
	require.NoError(t, err)
	expectedSeason, err := store.GetSeasonByID(ctx, season)
	require.NoError(t, err)
	expectedEpisode, err := store.GetEpisodeByID(ctx, episode)
	require.NoError(t, err)
	// Metadata child ownership remains restrictive: do not orphan its tree.
	requireConstraint(t, store.DeleteSeriesMetadata(ctx, seriesMetadata), sqlite3.ErrConstraintForeignKey)
	requireConstraint(t, store.DeleteSeasonMetadata(ctx, seasonMetadata), sqlite3.ErrConstraintForeignKey)
	require.NoError(t, store.DeleteEpisodeFile(ctx, episodeFile))
	expectedEpisode.Episode.EpisodeFileID = sql.NullInt64{}
	require.NoError(t, store.DeleteEpisodeMetadata(ctx, episodeMetadata))
	expectedEpisode.Episode.EpisodeMetadataID = sql.NullInt64{}
	gotEpisode, err := store.GetEpisodeByID(ctx, episode)
	require.NoError(t, err)
	assert.Equal(t, expectedEpisode, gotEpisode)
	require.NoError(t, store.DeleteSeasonMetadata(ctx, seasonMetadata))
	expectedSeason.Season.SeasonMetadataID = sql.NullInt64{}
	gotSeason, err := store.GetSeasonByID(ctx, season)
	require.NoError(t, err)
	assert.Equal(t, expectedSeason, gotSeason)
	require.NoError(t, store.DeleteSeriesMetadata(ctx, seriesMetadata))
	expectedSeries.Series.SeriesMetadataID = sql.NullInt64{}
	gotSeries, err := store.GetSeriesByID(ctx, series)
	require.NoError(t, err)
	assert.Equal(t, expectedSeries, gotSeries)
}

func TestMediaRejectsDanglingRelationalReferences(t *testing.T) {
	ctx := t.Context()
	store := initSqlite(t, ctx)
	for _, movie := range []sqlcdb.Movie{
		{QualityProfileID: -1},
		{QualityProfileID: 1, MovieMetadataID: sql.NullInt64{Int64: -1, Valid: true}},
		{QualityProfileID: 1, MovieFileID: sql.NullInt64{Int64: -1, Valid: true}},
	} {
		_, err := store.CreateMovie(ctx, movie, storage.MovieStateMissing)
		requireConstraint(t, err, sqlite3.ErrConstraintForeignKey)
	}
	for _, series := range []sqlcdb.Series{
		{QualityProfileID: -1},
		{QualityProfileID: 1, SeriesMetadataID: sql.NullInt64{Int64: -1, Valid: true}},
	} {
		_, err := store.CreateSeries(ctx, series, storage.SeriesStateMissing)
		requireConstraint(t, err, sqlite3.ErrConstraintForeignKey)
	}
	movies, err := store.ListMovies(ctx)
	require.NoError(t, err)
	assert.Empty(t, movies)
	series, err := store.ListSeries(ctx, storage.SeriesFilter{})
	require.NoError(t, err)
	assert.Empty(t, series)
}
