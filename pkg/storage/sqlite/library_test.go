package sqlite

import (
	"database/sql"
	"testing"

	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/stretchr/testify/require"
)

func TestLibraryMoviePagingAndCounts(t *testing.T) {
	ctx := t.Context()
	s := initSqlite(t, ctx)
	profile, err := s.CreateQualityProfile(ctx, sqlcdb.QualityProfile{Name: "library"})
	require.NoError(t, err)
	var created []int64
	for i, title := range []string{"Same", "Alpha", "Same", "Hidden"} {
		var metadata sql.NullInt64
		if i < 3 {
			id, err := s.CreateMovieMetadata(ctx, sqlcdb.MovieMetadatum{TmdbID: int64(i + 1), Title: title})
			require.NoError(t, err)
			metadata = sql.NullInt64{Int64: id, Valid: true}
		}
		state := storage.MovieStateMissing
		if i == 2 {
			state = storage.MovieStateDiscovered
		}
		id, err := s.CreateMovie(ctx, sqlcdb.Movie{MovieMetadataID: metadata, QualityProfileID: profile}, state)
		require.NoError(t, err)
		created = append(created, id)
	}
	p1, err := s.ListLibraryMovies(ctx, 0, 2, nil)
	require.NoError(t, err)
	require.Equal(t, []int64{created[1], created[0]}, []int64{p1[0].Movie.ID, p1[1].Movie.ID})
	p2, err := s.ListLibraryMovies(ctx, 2, 2, nil)
	require.NoError(t, err)
	require.Equal(t, []int64{created[2]}, []int64{p2[0].Movie.ID})
	states := []storage.MovieState{storage.MovieStateMissing, storage.MovieStateDiscovered}
	n, err := s.CountLibraryMovies(ctx, states)
	require.NoError(t, err)
	require.EqualValues(t, 3, n)
	filtered, err := s.ListLibraryMovies(ctx, 1, 2, states)
	require.NoError(t, err)
	require.Equal(t, []sqlcdb.ListLibraryMoviesRow{{Movie: p1[1].Movie, MovieMetadatum: p1[1].MovieMetadatum, State: string(storage.MovieStateMissing)}, {Movie: p2[0].Movie, MovieMetadatum: p2[0].MovieMetadatum, State: string(storage.MovieStateDiscovered)}}, filtered)
	by, err := s.CountLibraryMoviesByState(ctx)
	require.ElementsMatch(t, []sqlcdb.CountLibraryMoviesByStateRow{{State: string(storage.MovieStateMissing), Count: 2}, {State: string(storage.MovieStateDiscovered), Count: 1}}, by)
	empty, err := s.ListLibraryMovies(ctx, 99, 2, states)
	require.NoError(t, err)
	require.Empty(t, empty)
	all, err := s.ListMovies(ctx)
	require.NoError(t, err)
	require.Len(t, all, 4)
}

func TestLibraryShowPagingAndCounts(t *testing.T) {
	ctx := t.Context()
	s := initSqlite(t, ctx)
	profile, err := s.CreateQualityProfile(ctx, sqlcdb.QualityProfile{Name: "library"})
	require.NoError(t, err)
	var created []int64
	for i, title := range []string{"Same", "Alpha", "Same", "Hidden"} {
		var metadata sql.NullInt64
		if i < 3 {
			id, err := s.CreateSeriesMetadata(ctx, sqlcdb.SeriesMetadatum{TmdbID: int64(i + 1), Title: title})
			require.NoError(t, err)
			metadata = sql.NullInt64{Int64: id, Valid: true}
		}
		state := storage.SeriesStateMissing
		if i == 2 {
			state = storage.SeriesStateDiscovered
		}
		id, err := s.CreateSeries(ctx, sqlcdb.Series{SeriesMetadataID: metadata, QualityProfileID: profile}, state)
		require.NoError(t, err)
		created = append(created, id)
	}
	p1, err := s.ListLibraryShows(ctx, 0, 2, nil)
	require.NoError(t, err)
	require.Equal(t, []int64{created[1], created[0]}, []int64{p1[0].Series.ID, p1[1].Series.ID})
	p2, err := s.ListLibraryShows(ctx, 2, 2, nil)
	require.NoError(t, err)
	require.Equal(t, []int64{created[2]}, []int64{p2[0].Series.ID})
	states := []storage.SeriesState{storage.SeriesStateMissing, storage.SeriesStateDiscovered}
	n, err := s.CountLibraryShows(ctx, states)
	require.NoError(t, err)
	require.EqualValues(t, 3, n)
	filtered, err := s.ListLibraryShows(ctx, 1, 2, states)
	require.NoError(t, err)
	require.Equal(t, []sqlcdb.ListLibraryShowsRow{{Series: p1[1].Series, SeriesMetadatum: p1[1].SeriesMetadatum, State: string(storage.SeriesStateMissing)}, {Series: p2[0].Series, SeriesMetadatum: p2[0].SeriesMetadatum, State: string(storage.SeriesStateDiscovered)}}, filtered)
	by, err := s.CountLibraryShowsByState(ctx)
	require.ElementsMatch(t, []sqlcdb.CountLibraryShowsByStateRow{{State: string(storage.SeriesStateMissing), Count: 2}, {State: string(storage.SeriesStateDiscovered), Count: 1}}, by)
	empty, err := s.ListLibraryShows(ctx, 99, 2, states)
	require.NoError(t, err)
	require.Empty(t, empty)
	all, err := s.ListSeries(ctx, storage.SeriesFilter{})
	require.NoError(t, err)
	require.Len(t, all, 4)
}
