package manager

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kasuboski/mediaz/config"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	mediaSqlite "github.com/kasuboski/mediaz/pkg/storage/sqlite"
	"github.com/kasuboski/mediaz/pkg/tmdb"
	"github.com/stretchr/testify/require"
)

func filesystemDiscoveryManager(t *testing.T, store storage.Storage, handler http.HandlerFunc) MediaManager {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client, err := tmdb.New(server.URL, "test-token")
	require.NoError(t, err)
	return New(client, nil, nil, store, nil, config.Manager{}, config.Config{})
}

func TestFilesystemMovieIdentityWiring(t *testing.T) {
	for _, tc := range []struct {
		name, folder                                string
		files                                       []string
		results                                     string
		owned, alreadyLinked, outsideFile, wantLink bool
	}{
		{"explicit folder ID without search", "Brothers {tmdb-42}", []string{"movie.mkv"}, "", false, false, false, true},
		{"explicit filename ID without search", "Brothers", []string{"Brothers {tmdb-42}.mkv"}, "", false, false, false, true},
		{"conflicting explicit IDs", "Brothers {tmdb-42}", []string{"Brothers {tmdb-43}.mkv"}, "", false, false, false, false},
		{"metadata owned by another movie", "Brothers {tmdb-42}", nil, "", true, false, false, false},
		{"known metadata never reidentified", "Brothers {tmdb-99}", []string{"Brothers {tmdb-43}.mkv"}, "", false, true, false, false},
		{"filename year selects remake", "Brothers", []string{"Brothers.2009.1080p.mkv"}, `[{"id":43,"title":"Brothers","release_date":"2024-01-01"},{"id":42,"title":"Brothers","release_date":"2009-01-01"}]`, false, false, false, true},
		{"yearless remakes ambiguous", "Brothers", []string{"movie.mkv"}, `[{"id":43,"title":"Brothers","release_date":"2024-01-01"},{"id":42,"title":"Brothers","release_date":"2009-01-01"}]`, false, false, false, false},
		{"conflicting folder and filename", "Brothers (2009)", []string{"Brothers.2024.1080p.mkv"}, `[{"id":43,"title":"Brothers","release_date":"2024-01-01"},{"id":42,"title":"Brothers","release_date":"2009-01-01"}]`, false, false, false, false},
		{"unrelated same year not first", "Brothers (2009)", nil, `[{"id":43,"title":"Wrong Movie","release_date":"2009-01-01"},{"id":42,"title":"Brothers","release_date":"2009-01-01"}]`, false, false, false, true},
		{"prefix folder files do not conflict", "Brothers {tmdb-42}", []string{"Brothers.mkv"}, "", false, false, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			store, err := mediaSqlite.New(ctx, ":memory:")
			require.NoError(t, err)
			metadataID, err := store.CreateMovieMetadata(ctx, sqlcdb.MovieMetadatum{TmdbID: 42, Title: "Brothers", ReleaseDate: sql.NullTime{Time: time.Date(2009, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true}})
			require.NoError(t, err)
			_, err = store.CreateMovieMetadata(ctx, sqlcdb.MovieMetadatum{TmdbID: 43, Title: "Brothers", ReleaseDate: sql.NullTime{Time: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true}})
			require.NoError(t, err)
			if tc.owned {
				_, err = store.CreateMovie(ctx, sqlcdb.Movie{Path: sql.NullString{String: "Existing copy", Valid: true}, MovieMetadataID: sql.NullInt64{Int64: metadataID, Valid: true}, QualityProfileID: 1}, storage.MovieStateDiscovered)
				require.NoError(t, err)
			}
			var fileID sql.NullInt64
			for _, file := range tc.files {
				id, err := store.CreateMovieFile(ctx, sqlcdb.MovieFile{RelativePath: sql.NullString{String: tc.folder + "/" + file, Valid: true}})
				require.NoError(t, err)
				if !fileID.Valid {
					fileID = sql.NullInt64{Int64: id, Valid: true}
				}
			}
			if tc.outsideFile {
				_, err := store.CreateMovieFile(ctx, sqlcdb.MovieFile{RelativePath: sql.NullString{String: tc.folder + " Extra/Other {tmdb-43}.mkv", Valid: true}})
				require.NoError(t, err)
			}
			movie := sqlcdb.Movie{Path: sql.NullString{String: tc.folder, Valid: true}, QualityProfileID: 1, MovieFileID: fileID}
			if tc.alreadyLinked {
				movie.MovieMetadataID = sql.NullInt64{Int64: metadataID, Valid: true}
			}
			movieID, err := store.CreateMovie(ctx, movie, storage.MovieStateDiscovered)
			require.NoError(t, err)
			before, err := store.GetMovie(ctx, movieID)
			require.NoError(t, err)
			var searches atomic.Int32
			m := filesystemDiscoveryManager(t, store, func(w http.ResponseWriter, r *http.Request) {
				searches.Add(1)
				if tc.results == "" {
					t.Errorf("unexpected request: %s", r.URL)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				if r.URL.Path != "/3/search/movie" || r.URL.Query().Get("query") != "Brothers" {
					t.Errorf("unexpected search: %s", r.URL)
				}
				_, _ = w.Write([]byte(`{"results":` + tc.results + `}`))
			})
			require.NoError(t, m.reconcileDiscoveredMovie(ctx, sqlcdb.ListMoviesByStatesRow(before)))
			if tc.wantLink {
				before.Movie.MovieMetadataID = sql.NullInt64{Int64: metadataID, Valid: true}
			}
			after, err := store.GetMovie(ctx, movieID)
			require.NoError(t, err)
			require.Equal(t, before, after)
			if tc.results == "" {
				require.Zero(t, searches.Load())
			} else {
				require.Equal(t, int32(1), searches.Load())
			}
		})
	}
}
