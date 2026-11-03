package sqlite

import (
	"context"
	"database/sql"
	"testing"

	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/stretchr/testify/require"
)

func TestSeriesTreeAtomicRollback(t *testing.T) {
	ctx := context.Background()
	s, err := New(ctx, ":memory:")
	require.NoError(t, err)
	mid, err := s.CreateSeriesMetadata(ctx, sqlcdb.SeriesMetadatum{TmdbID: 123, Title: "Atomic"})
	require.NoError(t, err)
	series := sqlcdb.Series{SeriesMetadataID: sql.NullInt64{Int64: mid, Valid: true}, QualityProfileID: 4, Monitored: 1}
	// A late episode foreign-key failure must roll back the series, season and transitions.
	_, err = s.CreateSeriesTree(ctx, series, storage.SeriesStateMissing, []sqlcdb.Season{{SeasonNumber: 1, Monitored: 1}}, [][]sqlcdb.Episode{{{EpisodeNumber: 1, QualityProfileID: sql.NullInt64{Int64: 99999, Valid: true}}}})
	require.Error(t, err)
	rows, err := s.ListSeries(ctx, storage.SeriesFilter{})
	require.NoError(t, err)
	require.Empty(t, rows)
	seasons, err := s.ListSeasons(ctx, storage.SeasonFilter{})
	require.NoError(t, err)
	require.Empty(t, seasons)
	episodes, err := s.ListEpisodes(ctx, storage.EpisodeFilter{})
	require.NoError(t, err)
	require.Empty(t, episodes)
	_, err = s.CreateSeriesTree(ctx, series, storage.SeriesStateMissing, []sqlcdb.Season{{SeasonNumber: 1}}, nil)
	require.Error(t, err)
	id, err := s.CreateSeriesTree(ctx, series, storage.SeriesStateMissing, []sqlcdb.Season{{SeasonNumber: 0, Monitored: 1}}, [][]sqlcdb.Episode{{{EpisodeNumber: 1, Monitored: 1}}})
	require.NoError(t, err)
	got, err := s.GetSeriesByID(ctx, id)
	require.NoError(t, err)
	series.ID = id
	series.Added = got.Series.Added
	require.Equal(t, sqlcdb.GetSeriesRow{Series: series, State: string(storage.SeriesStateMissing)}, got)
}
