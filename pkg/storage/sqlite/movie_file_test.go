package sqlite

import (
	"database/sql"
	"sort"
	"testing"
	"time"

	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// CreateMovieFile
// ============================================================================

func TestCreateMovieFile(t *testing.T) {
	t.Run("assigns autoincrement id and default utc date_added", func(t *testing.T) {
		ctx := t.Context()
		store := initSqlite(t, ctx)

		before := time.Now().UTC()
		first, err := store.CreateMovieFile(ctx, sqlcdb.MovieFile{Quality: "HDTV-1080p", Size: 1_000_000, Languages: "[]"})
		require.NoError(t, err)
		second, err := store.CreateMovieFile(ctx, sqlcdb.MovieFile{Quality: "HDTV-720p", Size: 500, Languages: "[]"})
		require.NoError(t, err)
		after := time.Now().UTC()

		assert.Equal(t, int64(1), first)
		assert.Equal(t, int64(2), second, "ids must autoincrement")

		files, err := store.ListMovieFiles(ctx)
		require.NoError(t, err)
		require.Len(t, files, 2)
		for _, file := range files {
			// date_added uses SQLite's CURRENT_TIMESTAMP, which has second
			// precision: flooring can only pull the real insert time earlier,
			// so the bounds allow exactly that and nothing else.
			assert.WithinRange(t, file.DateAdded, before.Truncate(time.Second), after)
		}
	})

	t.Run("persists every column", func(t *testing.T) {
		ctx := t.Context()
		store := initSqlite(t, ctx)

		id, err := store.CreateMovieFile(ctx, sqlcdb.MovieFile{
			Quality:          "Bluray-2160p",
			Size:             42_000_000_000,
			SceneName:        sql.NullString{String: "Great.Movie.2024.2160p.BluRay.x264", Valid: true},
			MediaInfo:        sql.NullString{String: "h264 4:2:0", Valid: true},
			ReleaseGroup:     sql.NullString{String: "GROUP", Valid: true},
			RelativePath:     sql.NullString{String: "Great Movie/Great Movie.mkv", Valid: true},
			Edition:          sql.NullString{String: "Extended", Valid: true},
			Languages:        `["eng"]`,
			IndexerFlags:     7,
			OriginalFilePath: sql.NullString{String: "/downloads/Great.Movie.mkv", Valid: true},
		})
		require.NoError(t, err)

		files, err := store.ListMovieFiles(ctx)
		require.NoError(t, err)
		require.Len(t, files, 1)
		got := files[0]
		require.False(t, got.DateAdded.IsZero())
		assert.Equal(t, []sqlcdb.MovieFile{{
			ID:               id,
			Quality:          "Bluray-2160p",
			Size:             42_000_000_000,
			DateAdded:        got.DateAdded,
			SceneName:        sql.NullString{String: "Great.Movie.2024.2160p.BluRay.x264", Valid: true},
			MediaInfo:        sql.NullString{String: "h264 4:2:0", Valid: true},
			ReleaseGroup:     sql.NullString{String: "GROUP", Valid: true},
			RelativePath:     sql.NullString{String: "Great Movie/Great Movie.mkv", Valid: true},
			Edition:          sql.NullString{String: "Extended", Valid: true},
			Languages:        `["eng"]`,
			IndexerFlags:     7,
			OriginalFilePath: sql.NullString{String: "/downloads/Great.Movie.mkv", Valid: true},
		}}, files)
	})
}

// ============================================================================
// GetMovieFilesByMovieName
// ============================================================================

func TestGetMovieFilesByMovieName(t *testing.T) {
	t.Run("returns storage.ErrNotFound when there are no movie files", func(t *testing.T) {
		ctx := t.Context()
		store := initSqlite(t, ctx)

		files, err := store.GetMovieFilesByMovieName(ctx, "Title")
		require.ErrorIs(t, err, storage.ErrNotFound)
		assert.Nil(t, files)
	})

	t.Run("returns storage.ErrNotFound when no file path matches the movie path", func(t *testing.T) {
		ctx := t.Context()
		store := initSqlite(t, ctx)

		_, err := store.CreateMovieFile(ctx, sqlcdb.MovieFile{Quality: "HDTV-720p", RelativePath: sql.NullString{String: "Other/Other.mkv", Valid: true}})
		require.NoError(t, err)
		_, err = store.CreateMovie(ctx, sqlcdb.Movie{Path: sql.NullString{String: "Title", Valid: true}, Monitored: 1, QualityProfileID: 1}, storage.MovieStateMissing)
		require.NoError(t, err)

		files, err := store.GetMovieFilesByMovieName(ctx, "Title")
		require.ErrorIs(t, err, storage.ErrNotFound)
		assert.Nil(t, files)
	})

	t.Run("returns files whose relative path is prefixed by the movie path", func(t *testing.T) {
		ctx := t.Context()
		store := initSqlite(t, ctx)

		fileID, err := store.CreateMovieFile(ctx, sqlcdb.MovieFile{
			Quality:      "HDTV-720p",
			Size:         123,
			RelativePath: sql.NullString{String: "Title/Title.mkv", Valid: true},
			Languages:    "[]",
		})
		require.NoError(t, err)

		_, err = store.CreateMovie(ctx, sqlcdb.Movie{Path: sql.NullString{String: "Title", Valid: true}, Monitored: 1, QualityProfileID: 1}, storage.MovieStateMissing)
		require.NoError(t, err)

		files, err := store.GetMovieFilesByMovieName(ctx, "Title")
		require.NoError(t, err)
		require.Len(t, files, 1)
		require.False(t, files[0].DateAdded.IsZero())
		assert.Equal(t, []sqlcdb.MovieFile{{
			ID:           fileID,
			Quality:      "HDTV-720p",
			Size:         123,
			DateAdded:    files[0].DateAdded,
			RelativePath: sql.NullString{String: "Title/Title.mkv", Valid: true},
			Languages:    "[]",
		}}, files)
	})

	t.Run("returns every file of the movie and only that movie's files", func(t *testing.T) {
		ctx := t.Context()
		store := initSqlite(t, ctx)

		alphaFileIDs := make([]int64, 0, 2)
		for _, name := range []string{"Alpha/Alpha.mkv", "Alpha/extras/Alpha-trailer.mkv"} {
			id, err := store.CreateMovieFile(ctx, sqlcdb.MovieFile{
				Quality:      "HDTV-720p",
				RelativePath: sql.NullString{String: name, Valid: true},
				Languages:    "[]",
			})
			require.NoError(t, err)
			alphaFileIDs = append(alphaFileIDs, id)
		}
		_, err := store.CreateMovie(ctx, sqlcdb.Movie{Path: sql.NullString{String: "Alpha", Valid: true}, Monitored: 1, QualityProfileID: 1}, storage.MovieStateMissing)
		require.NoError(t, err)

		_, err = store.CreateMovieFile(ctx, sqlcdb.MovieFile{Quality: "HDTV-720p", RelativePath: sql.NullString{String: "Beta/Beta.mkv", Valid: true}})
		require.NoError(t, err)
		_, err = store.CreateMovie(ctx, sqlcdb.Movie{Path: sql.NullString{String: "Beta", Valid: true}, Monitored: 1, QualityProfileID: 1}, storage.MovieStateMissing)
		require.NoError(t, err)

		files, err := store.GetMovieFilesByMovieName(ctx, "Alpha")
		require.NoError(t, err)
		require.Len(t, files, 2)
		sort.Slice(files, func(i, j int) bool { return files[i].ID < files[j].ID })
		for i, file := range files {
			assert.Equal(t, alphaFileIDs[i], file.ID)
		}

		betaFiles, err := store.GetMovieFilesByMovieName(ctx, "Beta")
		require.NoError(t, err)
		require.Len(t, betaFiles, 1)
		assert.Equal(t, "Beta/Beta.mkv", betaFiles[0].RelativePath.String)
	})
}

// ============================================================================
// ListMovieFiles
// ============================================================================

func TestListMovieFiles(t *testing.T) {
	t.Run("returns empty list when there are no movie files", func(t *testing.T) {
		ctx := t.Context()
		store := initSqlite(t, ctx)

		files, err := store.ListMovieFiles(ctx)
		require.NoError(t, err)
		assert.Empty(t, files)
	})

	t.Run("returns all files ordered by id ascending", func(t *testing.T) {
		ctx := t.Context()
		store := initSqlite(t, ctx)

		want := make([]sqlcdb.MovieFile, 0, 3)
		for _, relPath := range []string{"a.mkv", "b.mkv", "c.mkv"} {
			id, err := store.CreateMovieFile(ctx, sqlcdb.MovieFile{
				Quality:      "HDTV-720p",
				Size:         int64(len(relPath)),
				RelativePath: sql.NullString{String: relPath, Valid: true},
				Languages:    "[]",
			})
			require.NoError(t, err)
			want = append(want, sqlcdb.MovieFile{
				ID:           id,
				Quality:      "HDTV-720p",
				Size:         int64(len(relPath)),
				RelativePath: sql.NullString{String: relPath, Valid: true},
				Languages:    "[]",
			})
		}

		files, err := store.ListMovieFiles(ctx)
		require.NoError(t, err)
		require.Len(t, files, len(want))
		for i := range want {
			require.False(t, files[i].DateAdded.IsZero())
			want[i].DateAdded = files[i].DateAdded
		}
		assert.Equal(t, want, files)
	})
}

// ============================================================================
// DeleteMovieFile
// ============================================================================

func TestDeleteMovieFile(t *testing.T) {
	t.Run("deletes only the targeted file", func(t *testing.T) {
		ctx := t.Context()
		store := initSqlite(t, ctx)

		first, err := store.CreateMovieFile(ctx, sqlcdb.MovieFile{Quality: "HDTV-720p", RelativePath: sql.NullString{String: "first.mkv", Valid: true}})
		require.NoError(t, err)
		second, err := store.CreateMovieFile(ctx, sqlcdb.MovieFile{Quality: "HDTV-720p", RelativePath: sql.NullString{String: "second.mkv", Valid: true}})
		require.NoError(t, err)

		require.NoError(t, store.DeleteMovieFile(ctx, first))

		files, err := store.ListMovieFiles(ctx)
		require.NoError(t, err)
		require.Len(t, files, 1)
		require.False(t, files[0].DateAdded.IsZero())
		assert.Equal(t, []sqlcdb.MovieFile{{
			ID:           second,
			Quality:      "HDTV-720p",
			RelativePath: sql.NullString{String: "second.mkv", Valid: true},
			DateAdded:    files[0].DateAdded,
		}}, files)
	})

	t.Run("deleting an unknown id is a no-op without error", func(t *testing.T) {
		ctx := t.Context()
		store := initSqlite(t, ctx)

		fileID, err := store.CreateMovieFile(ctx, sqlcdb.MovieFile{Quality: "HDTV-720p"})
		require.NoError(t, err)

		require.NoError(t, store.DeleteMovieFile(ctx, fileID+100))

		files, err := store.ListMovieFiles(ctx)
		require.NoError(t, err)
		assert.Len(t, files, 1)
	})

	t.Run("deleted file is no longer returned by movie name lookup", func(t *testing.T) {
		ctx := t.Context()
		store := initSqlite(t, ctx)

		fileID, err := store.CreateMovieFile(ctx, sqlcdb.MovieFile{Quality: "HDTV-720p", RelativePath: sql.NullString{String: "Title/Title.mkv", Valid: true}})
		require.NoError(t, err)
		_, err = store.CreateMovie(ctx, sqlcdb.Movie{Path: sql.NullString{String: "Title", Valid: true}, Monitored: 1, QualityProfileID: 1}, storage.MovieStateMissing)
		require.NoError(t, err)

		require.NoError(t, store.DeleteMovieFile(ctx, fileID))

		files, err := store.GetMovieFilesByMovieName(ctx, "Title")
		require.ErrorIs(t, err, storage.ErrNotFound)
		assert.Nil(t, files)
	})
}
