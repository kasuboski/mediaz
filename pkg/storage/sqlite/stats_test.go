package sqlite

import (
	"context"
	"testing"

	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetLibraryStats(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		ctx := context.Background()
		store := initSqlite(t, ctx)

		got, err := store.GetLibraryStats(ctx)

		require.NoError(t, err)
		assert.Equal(t, &storage.LibraryStats{
			Movies: storage.MovieStats{ByState: map[storage.MovieState]int{}},
			TV:     storage.TVStats{ByState: map[storage.SeriesState]int{}},
		}, got)
	})

	t.Run("movies only", func(t *testing.T) {
		ctx := context.Background()
		store := initSqlite(t, ctx)

		p1, p2, p3 := "path/1", "path/2", "path/3"

		_, err := store.CreateMovie(ctx, storage.Movie{Movie: model.Movie{Path: &p1, Monitored: 1}}, storage.MovieStateMissing)
		require.NoError(t, err)
		_, err = store.CreateMovie(ctx, storage.Movie{Movie: model.Movie{Path: &p2, Monitored: 1}}, storage.MovieStateMissing)
		require.NoError(t, err)

		id3, err := store.CreateMovie(ctx, storage.Movie{Movie: model.Movie{Path: &p3, Monitored: 1}}, storage.MovieStateMissing)
		require.NoError(t, err)
		require.NoError(t, store.UpdateMovieState(ctx, id3, storage.MovieStateDiscovered, nil))

		got, err := store.GetLibraryStats(ctx)

		require.NoError(t, err)
		assert.Equal(t, &storage.LibraryStats{
			Movies: storage.MovieStats{
				Total: 3,
				ByState: map[storage.MovieState]int{
					storage.MovieStateMissing:    2,
					storage.MovieStateDiscovered: 1,
				},
			},
			TV: storage.TVStats{ByState: map[storage.SeriesState]int{}},
		}, got)
	})

	t.Run("tv only", func(t *testing.T) {
		ctx := context.Background()
		store := initSqlite(t, ctx)

		p1, p2, p3 := "path/1", "path/2", "path/3"

		_, err := store.CreateSeries(ctx, storage.Series{Series: model.Series{Path: &p1, Monitored: 1, QualityProfileID: 1}}, storage.SeriesStateMissing)
		require.NoError(t, err)

		id2, err := store.CreateSeries(ctx, storage.Series{Series: model.Series{Path: &p2, Monitored: 1, QualityProfileID: 1}}, storage.SeriesStateMissing)
		require.NoError(t, err)
		require.NoError(t, store.UpdateSeriesState(ctx, id2, storage.SeriesStateDiscovered, nil))

		id3, err := store.CreateSeries(ctx, storage.Series{Series: model.Series{Path: &p3, Monitored: 1, QualityProfileID: 1}}, storage.SeriesStateMissing)
		require.NoError(t, err)
		require.NoError(t, store.UpdateSeriesState(ctx, id3, storage.SeriesStateDiscovered, nil))

		got, err := store.GetLibraryStats(ctx)

		require.NoError(t, err)
		assert.Equal(t, &storage.LibraryStats{
			Movies: storage.MovieStats{ByState: map[storage.MovieState]int{}},
			TV: storage.TVStats{
				Total: 3,
				ByState: map[storage.SeriesState]int{
					storage.SeriesStateMissing:    1,
					storage.SeriesStateDiscovered: 2,
				},
			},
		}, got)
	})

	t.Run("movies and tv", func(t *testing.T) {
		ctx := context.Background()
		store := initSqlite(t, ctx)

		mp1, mp2 := "movie/1", "movie/2"
		sp1, sp2 := "series/1", "series/2"

		_, err := store.CreateMovie(ctx, storage.Movie{Movie: model.Movie{Path: &mp1, Monitored: 1}}, storage.MovieStateMissing)
		require.NoError(t, err)

		id2, err := store.CreateMovie(ctx, storage.Movie{Movie: model.Movie{Path: &mp2, Monitored: 1}}, storage.MovieStateMissing)
		require.NoError(t, err)
		require.NoError(t, store.UpdateMovieState(ctx, id2, storage.MovieStateDiscovered, nil))

		_, err = store.CreateSeries(ctx, storage.Series{Series: model.Series{Path: &sp1, Monitored: 1, QualityProfileID: 1}}, storage.SeriesStateMissing)
		require.NoError(t, err)

		id4, err := store.CreateSeries(ctx, storage.Series{Series: model.Series{Path: &sp2, Monitored: 1, QualityProfileID: 1}}, storage.SeriesStateMissing)
		require.NoError(t, err)
		require.NoError(t, store.UpdateSeriesState(ctx, id4, storage.SeriesStateDownloading, nil))

		got, err := store.GetLibraryStats(ctx)

		require.NoError(t, err)
		assert.Equal(t, &storage.LibraryStats{
			Movies: storage.MovieStats{
				Total: 2,
				ByState: map[storage.MovieState]int{
					storage.MovieStateMissing:    1,
					storage.MovieStateDiscovered: 1,
				},
			},
			TV: storage.TVStats{
				Total: 2,
				ByState: map[storage.SeriesState]int{
					storage.SeriesStateMissing:     1,
					storage.SeriesStateDownloading: 1,
				},
			},
		}, got)
	})
}
