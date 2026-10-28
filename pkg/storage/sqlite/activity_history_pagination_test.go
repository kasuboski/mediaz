package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/stretchr/testify/require"
)

// SQLite's driver scans LEAD(created_at) into interface{} as either []byte or
// string, while a terminal SQL NULL is nil. Compare its textual value to the
// exact timestamp written to SQLite rather than assuming a generated Go type.
func assertNextHistoryTime(t *testing.T, got interface{}, want time.Time) {
	t.Helper()
	var text string
	switch value := got.(type) {
	case []byte:
		text = string(value)
	case string:
		text = value
	default:
		require.Failf(t, "unexpected next_created_at type", "%T: %#v", got, got)
	}
	require.Equal(t, want.UTC().Format("2006-01-02 15:04:05.000000000"), text)
}

func TestEntityHistoryPagePagination(t *testing.T) {
	tests := []struct {
		name string
		run  func(context.Context, *SQLite) (int64, func(int64, int64) ([]int64, []interface{}, error))
	}{
		{"movie", func(ctx context.Context, s *SQLite) (int64, func(int64, int64) ([]int64, []interface{}, error)) {
			id, err := s.CreateMovie(ctx, sqlcdb.Movie{QualityProfileID: 1}, storage.MovieStateMissing)
			require.NoError(t, err)
			_, err = s.CreateMovieMetadata(ctx, sqlcdb.MovieMetadatum{TmdbID: 1, Title: "history"})
			require.NoError(t, err)
			require.NoError(t, s.LinkMovieMetadata(ctx, id, 1))
			for _, state := range []storage.MovieState{storage.MovieStateDownloading, storage.MovieStateDownloaded} {
				require.NoError(t, s.UpdateMovieState(ctx, id, state, nil))
			}
			return id, func(off, lim int64) ([]int64, []interface{}, error) {
				rows, err := s.GetMovieHistoryPage(ctx, sqlcdb.GetMovieHistoryPageParams{EntityID: id, Offs: off, Lim: lim})
				keys := []int64{}
				next := []interface{}{}
				for _, r := range rows {
					keys = append(keys, r.SortKey)
					next = append(next, r.NextCreatedAt)
				}
				return keys, next, err
			}
		}},
		{"series", func(ctx context.Context, s *SQLite) (int64, func(int64, int64) ([]int64, []interface{}, error)) {
			id, err := s.CreateSeries(ctx, sqlcdb.Series{QualityProfileID: 1}, storage.SeriesStateMissing)
			require.NoError(t, err)
			for _, st := range []storage.SeriesState{storage.SeriesStateDownloading, storage.SeriesStateCompleted} {
				require.NoError(t, s.UpdateSeriesState(ctx, id, st, nil))
			}
			return id, func(o, l int64) ([]int64, []interface{}, error) {
				r, e := s.GetSeriesHistoryPage(ctx, sqlcdb.GetSeriesHistoryPageParams{EntityID: id, Offs: o, Lim: l})
				k := []int64{}
				n := []interface{}{}
				for _, x := range r {
					k = append(k, x.SortKey)
					n = append(n, x.NextCreatedAt)
				}
				return k, n, e
			}
		}},
		{"season", func(ctx context.Context, s *SQLite) (int64, func(int64, int64) ([]int64, []interface{}, error)) {
			sid, e := s.CreateSeries(ctx, sqlcdb.Series{QualityProfileID: 1}, storage.SeriesStateMissing)
			require.NoError(t, e)
			id, e := s.CreateSeason(ctx, sqlcdb.Season{SeriesID: sid, SeasonNumber: 1}, storage.SeasonStateMissing)
			require.NoError(t, e)
			for _, st := range []storage.SeasonState{storage.SeasonStateDownloading, storage.SeasonStateCompleted} {
				require.NoError(t, s.UpdateSeasonState(ctx, id, st, nil))
			}
			return id, func(o, l int64) ([]int64, []interface{}, error) {
				r, e := s.GetSeasonHistoryPage(ctx, sqlcdb.GetSeasonHistoryPageParams{EntityID: id, Offs: o, Lim: l})
				k := []int64{}
				n := []interface{}{}
				for _, x := range r {
					k = append(k, x.SortKey)
					n = append(n, x.NextCreatedAt)
				}
				return k, n, e
			}
		}},
		{"episode", func(ctx context.Context, s *SQLite) (int64, func(int64, int64) ([]int64, []interface{}, error)) {
			sid, e := s.CreateSeries(ctx, sqlcdb.Series{QualityProfileID: 1}, storage.SeriesStateMissing)
			require.NoError(t, e)
			season, e := s.CreateSeason(ctx, sqlcdb.Season{SeriesID: sid, SeasonNumber: 1}, storage.SeasonStateMissing)
			require.NoError(t, e)
			id, e := s.CreateEpisode(ctx, sqlcdb.Episode{SeasonID: season, EpisodeNumber: 1}, storage.EpisodeStateMissing)
			require.NoError(t, e)
			for _, st := range []storage.EpisodeState{storage.EpisodeStateDownloading, storage.EpisodeStateDownloaded} {
				require.NoError(t, s.UpdateEpisodeState(ctx, id, st, nil))
			}
			return id, func(o, l int64) ([]int64, []interface{}, error) {
				r, e := s.GetEpisodeHistoryPage(ctx, sqlcdb.GetEpisodeHistoryPageParams{EntityID: id, Offs: o, Lim: l})
				k := []int64{}
				n := []interface{}{}
				for _, x := range r {
					k = append(k, x.SortKey)
					n = append(n, x.NextCreatedAt)
				}
				return k, n, e
			}
		}},
		{"job", func(ctx context.Context, s *SQLite) (int64, func(int64, int64) ([]int64, []interface{}, error)) {
			id, e := s.CreateJob(ctx, sqlcdb.Job{Type: "history"}, storage.JobStatePending)
			require.NoError(t, e)
			for _, st := range []storage.JobState{storage.JobStateRunning, storage.JobStateDone} {
				require.NoError(t, s.UpdateJobState(ctx, id, st, nil))
			}
			return id, func(o, l int64) ([]int64, []interface{}, error) {
				r, e := s.GetJobHistoryPage(ctx, sqlcdb.GetJobHistoryPageParams{EntityID: id, Offs: o, Lim: l})
				k := []int64{}
				n := []interface{}{}
				for _, x := range r {
					k = append(k, x.SortKey)
					n = append(n, x.NextCreatedAt)
				}
				return k, n, e
			}
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			s := initSqlite(t, ctx).(*SQLite)
			id, page := tc.run(ctx, s)
			base := time.Date(2024, 6, 15, 12, 0, 0, 123456789, time.UTC)
			transition := map[string][2]string{"movie": {"movie_transition", "movie_id"}, "series": {"series_transition", "series_id"}, "season": {"season_transition", "season_id"}, "episode": {"episode_transition", "episode_id"}, "job": {"job_transition", "job_id"}}[tc.name]
			table, column := transition[0], transition[1]
			for i := 1; i <= 3; i++ {
				stamp := base.Add(time.Duration(i*10) * time.Second).Format("2006-01-02 15:04:05.000000000")
				_, err := s.db.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET created_at=?, updated_at=? WHERE %s=? AND sort_key=?", table, column), stamp, stamp, id, i)
				require.NoError(t, err)
			}
			// Ensure the stored timestamps (including fractional seconds) are strictly
			// ordered and inspect the precise page-boundary lookahead.
			first, next, err := page(0, 2)
			require.NoError(t, err)
			require.Equal(t, []int64{1, 2}, first)
			require.Len(t, next, 2)
			assertNextHistoryTime(t, next[1], base.Add(30*time.Second))
			second, last, err := page(2, 2)
			require.NoError(t, err)
			require.Equal(t, []int64{3}, second)
			require.Nil(t, last[0])
			again, _, err := page(0, 2)
			require.NoError(t, err)
			require.Equal(t, first, again)
			all, _, err := page(0, 10)
			require.NoError(t, err)
			require.Equal(t, []int64{1, 2, 3}, all)
			empty, _, err := page(3, 2)
			require.NoError(t, err)
			require.Empty(t, empty)
		})
	}
}

var _ = sql.NullString{}
