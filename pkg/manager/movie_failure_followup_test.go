package manager

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/kasuboski/mediaz/config"
	"github.com/kasuboski/mediaz/pkg/download"
	downloadMock "github.com/kasuboski/mediaz/pkg/download/mocks"
	httpMock "github.com/kasuboski/mediaz/pkg/http/mocks"
	"github.com/kasuboski/mediaz/pkg/indexer"
	indexerMock "github.com/kasuboski/mediaz/pkg/indexer/mocks"
	"github.com/kasuboski/mediaz/pkg/library"
	libraryMock "github.com/kasuboski/mediaz/pkg/library/mocks"
	"github.com/kasuboski/mediaz/pkg/prowlarr"
	"github.com/kasuboski/mediaz/pkg/ptr"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	mediaSqlite "github.com/kasuboski/mediaz/pkg/storage/sqlite"
	"github.com/kasuboski/mediaz/pkg/tmdb"
	"github.com/oapi-codegen/nullable"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// A second connection permits precise SQLite failures and inspection of inactive
// transitions too; checking just GetMovie would miss accidental history writes.
func movieFollowupStore(t *testing.T) (storage.Storage, *sql.DB) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "movie.db")
	store, err := mediaSqlite.New(context.Background(), path)
	require.NoError(t, err)
	db, err := sql.Open("sqlite3", path+"?_foreign_keys=on&_busy_timeout=5000")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	return store, db
}

func movieFollowupHistory(t *testing.T, db *sql.DB, id int64) []sqlcdb.MovieTransition {
	t.Helper()
	rows, err := db.Query(`SELECT id, movie_id, to_state, from_state, most_recent, sort_key, download_client_id, download_id, created_at, updated_at FROM movie_transition WHERE movie_id = ? ORDER BY sort_key, id`, id)
	require.NoError(t, err)
	defer rows.Close()
	history := make([]sqlcdb.MovieTransition, 0)
	for rows.Next() {
		var row sqlcdb.MovieTransition
		require.NoError(t, rows.Scan(&row.ID, &row.MovieID, &row.ToState, &row.FromState, &row.MostRecent, &row.SortKey, &row.DownloadClientID, &row.DownloadID, &row.CreatedAt, &row.UpdatedAt))
		history = append(history, row)
	}
	require.NoError(t, rows.Err())
	return history
}

func movieFollowupExec(t *testing.T, db *sql.DB, query string) {
	t.Helper()
	_, err := db.Exec(query)
	require.NoError(t, err)
}

func TestMovieFailureFollowup_DiscoveredMetadataPersistence(t *testing.T) {
	for _, fail := range []bool{false, true} {
		name := "success"
		if fail {
			name = "link failure"
		}
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			store, db := movieFollowupStore(t)
			metadataID, err := store.CreateMovieMetadata(ctx, sqlcdb.MovieMetadatum{TmdbID: 1234, Title: "test movie"})
			require.NoError(t, err)
			id, err := store.CreateMovie(ctx, sqlcdb.Movie{Monitored: 1, QualityProfileID: 1, Path: sql.NullString{String: "test movie", Valid: true}}, storage.MovieStateDiscovered)
			require.NoError(t, err)
			before, err := store.GetMovie(ctx, id)
			require.NoError(t, err)
			history := movieFollowupHistory(t, db, id)
			if fail {
				movieFollowupExec(t, db, `CREATE TRIGGER fail_link BEFORE UPDATE OF movie_metadata_id ON movie BEGIN SELECT RAISE(ABORT, 'injected link failure'); END`)
			}
			ctrl := gomock.NewController(t)
			httpClient := httpMock.NewMockHTTPClient(ctrl)
			httpClient.EXPECT().Do(gomock.Any()).Return(&http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"results":[{"id":1234,"title":"test movie"}]}`))}, nil)
			client, err := tmdb.New("https://api.themoviedb.org", "test", tmdb.WithHTTPClient(httpClient))
			require.NoError(t, err)
			m := New(client, nil, nil, store, nil, config.Manager{}, config.Config{})
			err = m.reconcileDiscoveredMovie(ctx, sqlcdb.ListMoviesByStatesRow(before))
			want := before
			if fail {
				require.EqualError(t, err, "failed to update movie: injected link failure")
			} else {
				require.NoError(t, err)
				want.Movie.MovieMetadataID = sql.NullInt64{Int64: metadataID, Valid: true}
			}
			got, err := store.GetMovie(ctx, id)
			require.NoError(t, err)
			require.Equal(t, want, got)
			require.Equal(t, history, movieFollowupHistory(t, db, id))
			linked, err := store.GetMovieByMetadataID(ctx, metadataID)
			if fail {
				require.ErrorIs(t, err, storage.ErrNotFound)
			} else {
				require.NoError(t, err)
				require.Equal(t, sqlcdb.GetMovieByMetadataIDRow(want), linked)
			}
		})
	}
}

func TestMovieFailureFollowup_MissingPreservesRowAndHistory(t *testing.T) {
	for _, scenario := range []string{"no releases", "all rejected", "search source error", "indexer search error", "download add error"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			ctrl := gomock.NewController(t)
			store, db := movieFollowupStore(t)
			metadataID, err := store.CreateMovieMetadata(ctx, sqlcdb.MovieMetadatum{TmdbID: 1234, Title: "test movie", Runtime: 120})
			require.NoError(t, err)
			id, err := store.CreateMovie(ctx, sqlcdb.Movie{Monitored: 1, QualityProfileID: 1, Path: sql.NullString{String: "test movie", Valid: true}, MovieMetadataID: sql.NullInt64{Int64: metadataID, Valid: true}}, storage.MovieStateMissing)
			require.NoError(t, err)
			before, err := store.GetMovie(ctx, id)
			require.NoError(t, err)
			history := movieFollowupHistory(t, db, id)
			sourceID, err := store.CreateIndexerSource(ctx, sqlcdb.IndexerSource{Name: "source", Implementation: "prowlarr", Scheme: "http", Host: "test", Enabled: true})
			require.NoError(t, err)
			client := sqlcdb.DownloadClient{Implementation: "transmission", Type: "torrent", Scheme: "http", Host: "test", Port: 8080}
			client.ID, err = store.CreateDownloadClient(ctx, client)
			require.NoError(t, err)
			source := indexerMock.NewMockIndexerSource(ctrl)
			factory := indexerMock.NewMockFactory(ctrl)
			source.EXPECT().ListIndexers(ctx).Return([]indexer.SourceIndexer{{ID: indexer.ProwlarrID(1), Name: "indexer", Priority: 1}}, nil)
			factory.EXPECT().NewIndexerSource(gomock.Any()).Return(source, nil)
			injected := errors.New("injected search failure")
			var release *prowlarr.ReleaseResource
			if scenario == "search source error" {
				factory.EXPECT().NewIndexerSource(gomock.Any()).Return(nil, injected)
			} else {
				factory.EXPECT().NewIndexerSource(gomock.Any()).Return(source, nil)
				var releases []*prowlarr.ReleaseResource
				var searchErr error
				if scenario == "all rejected" || scenario == "download add error" {
					protocol := prowlarr.DownloadProtocolTorrent
					if scenario == "all rejected" {
						protocol = prowlarr.DownloadProtocolUsenet
					}
					release = &prowlarr.ReleaseResource{ID: ptr.To(int32(123)), Title: nullable.NewNullableWithValue("test movie"), Size: ptr.To(int64(23 * 1024 * 1024 * 1024)), Seeders: nullable.NewNullableWithValue(int32(23)), Protocol: &protocol}
					releases = []*prowlarr.ReleaseResource{release}
				}
				if scenario == "indexer search error" {
					searchErr = injected
				}
				source.EXPECT().Search(gomock.Any(), indexer.ProwlarrID(1), MovieCategories, indexer.SearchOptions{Query: "test movie", Type: ptr.To(indexer.TypeMovie)}).Return(releases, searchErr)
			}
			downloads := downloadMock.NewMockFactory(ctrl)
			dc := downloadMock.NewMockDownloadClient(ctrl)
			addErr := errors.New("injected add failure")
			// No expectations in the other cases: even building a client is forbidden.
			if scenario == "download add error" {
				downloads.EXPECT().NewDownloadClient(client).Return(dc, nil)
				dc.EXPECT().Add(ctx, download.AddRequest{Release: release}).Return(download.Status{ID: "must-not-persist"}, addErr)
			}
			m := New(nil, factory, nil, store, downloads, config.Manager{}, config.Config{})
			require.NoError(t, m.RefreshIndexerSource(ctx, sourceID))
			indexers, err := m.listIndexersInternal(ctx)
			require.NoError(t, err)
			err = m.reconcileMissingMovie(ctx, sqlcdb.ListMoviesByStatesRow(before), newReconcileSnapshot(indexers, []sqlcdb.DownloadClient{client}))
			switch scenario {
			case "search source error":
				require.ErrorIs(t, err, injected)
			case "download add error":
				require.ErrorIs(t, err, addErr)
				require.EqualError(t, err, "failed to add movie download request: injected add failure")
			default:
				// searchIndexerSource currently logs and swallows individual Search errors.
				require.NoError(t, err)
			}
			got, err := store.GetMovie(ctx, id)
			require.NoError(t, err)
			require.Equal(t, before, got)
			require.Equal(t, history, movieFollowupHistory(t, db, id))
		})
	}
}

func TestMovieFailureFollowup_PastReleaseTransitionsToMissing(t *testing.T) {
	ctx := context.Background()
	store, db := movieFollowupStore(t)
	now := time.Date(2025, 5, 1, 12, 0, 0, 0, time.UTC)
	metadataID, err := store.CreateMovieMetadata(ctx, sqlcdb.MovieMetadatum{TmdbID: 1234, Title: "released movie", ReleaseDate: sql.NullTime{Time: now.Add(-24 * time.Hour), Valid: true}})
	require.NoError(t, err)
	id, err := store.CreateMovie(ctx, sqlcdb.Movie{Monitored: 1, QualityProfileID: 1, MovieMetadataID: sql.NullInt64{Int64: metadataID, Valid: true}}, storage.MovieStateUnreleased)
	require.NoError(t, err)
	before, err := store.GetMovie(ctx, id)
	require.NoError(t, err)
	history := movieFollowupHistory(t, db, id)
	require.Len(t, history, 1)
	snapshot := newReconcileSnapshot(nil, nil)
	snapshot.time = now
	m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
	require.NoError(t, m.ReconcileUnreleasedMovies(ctx, snapshot))
	got, err := store.GetMovie(ctx, id)
	require.NoError(t, err)
	want := before
	want.State = string(storage.MovieStateMissing)
	require.Equal(t, want, got)
	after := movieFollowupHistory(t, db, id)
	require.Len(t, after, 2)
	old := history[0]
	old.MostRecent = false
	// Deactivation legitimately updates its timestamp; all other fields survive.
	require.True(t, after[0].UpdatedAt.Valid)
	old.UpdatedAt = after[0].UpdatedAt
	require.Equal(t, old, after[0])
	require.Positive(t, after[1].ID)
	require.True(t, after[1].CreatedAt.Valid)
	require.True(t, after[1].UpdatedAt.Valid)
	require.Equal(t, sqlcdb.MovieTransition{ID: after[1].ID, MovieID: id, ToState: string(storage.MovieStateMissing), FromState: sql.NullString{String: string(storage.MovieStateUnreleased), Valid: true}, MostRecent: true, SortKey: old.SortKey + 1, CreatedAt: after[1].CreatedAt, UpdatedAt: after[1].UpdatedAt}, after[1])
	// A second public reconcile must not append a duplicate transition.
	require.NoError(t, m.ReconcileUnreleasedMovies(ctx, snapshot))
	require.Equal(t, after, movieFollowupHistory(t, db, id))
}

func TestMovieFailureFollowup_CompletedMultiFileFailures(t *testing.T) {
	for _, scenario := range []string{"later library failure", "later file persistence failure", "final state failure"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			ctrl := gomock.NewController(t)
			store, db := movieFollowupStore(t)
			metadataID, err := store.CreateMovieMetadata(ctx, sqlcdb.MovieMetadatum{TmdbID: 1234, Title: "my-movie"})
			require.NoError(t, err)
			client := sqlcdb.DownloadClient{Implementation: "transmission", Type: "torrent", Scheme: "http", Host: "test", Port: 8080}
			client.ID, err = store.CreateDownloadClient(ctx, client)
			require.NoError(t, err)
			id, err := store.CreateMovie(ctx, sqlcdb.Movie{Monitored: 1, QualityProfileID: 1, Path: sql.NullString{String: "my-movie", Valid: true}, MovieMetadataID: sql.NullInt64{Int64: metadataID, Valid: true}}, storage.MovieStateMissing)
			require.NoError(t, err)
			require.NoError(t, store.UpdateMovieState(ctx, id, storage.MovieStateDownloading, &storage.TransitionStateMetadata{DownloadID: ptr.To("download-123"), DownloadClientID: &client.ID}))
			before, err := store.GetMovie(ctx, id)
			require.NoError(t, err)
			history := movieFollowupHistory(t, db, id)
			if scenario == "later file persistence failure" {
				movieFollowupExec(t, db, `CREATE TRIGGER fail_second_file BEFORE INSERT ON movie_file WHEN NEW.relative_path = 'my-movie/part2.mkv' BEGIN SELECT RAISE(ABORT, 'injected file failure'); END`)
			}
			if scenario == "final state failure" {
				movieFollowupExec(t, db, `CREATE TRIGGER fail_downloaded BEFORE INSERT ON movie_transition WHEN NEW.to_state = 'downloaded' BEGIN SELECT RAISE(ABORT, 'injected state failure'); END`)
			}
			factory := downloadMock.NewMockFactory(ctrl)
			dc := downloadMock.NewMockDownloadClient(ctrl)
			factory.EXPECT().NewDownloadClient(client).Return(dc, nil)
			dc.EXPECT().Get(ctx, download.GetRequest{ID: "download-123"}).Return(download.Status{ID: "download-123", Done: true, FilePaths: []string{"/downloads/part1.mkv", "/downloads/part2.mkv", "/downloads/part3.mkv"}}, nil)
			lib := libraryMock.NewMockLibrary(ctrl)
			injected := errors.New("injected library failure")
			var previous *gomock.Call
			count := 2
			if scenario == "final state failure" {
				count = 3
			}
			for i := 1; i <= count; i++ {
				part := strconv.Itoa(i)
				var libErr error
				if scenario == "later library failure" && i == 2 {
					libErr = injected
				}
				call := lib.EXPECT().AddMovie(ctx, "my-movie", "/downloads/part"+part+".mkv").Return(library.MovieFile{RelativePath: "my-movie/part" + part + ".mkv", Size: int64(i * 1024)}, libErr)
				if previous != nil {
					call.After(previous)
				}
				previous = call
			}
			m := New(nil, nil, lib, store, factory, config.Manager{}, config.Config{})
			err = m.reconcileDownloadingMovie(ctx, sqlcdb.ListMoviesByStatesRow(before), newReconcileSnapshot(nil, []sqlcdb.DownloadClient{client}))
			switch scenario {
			case "later library failure":
				require.ErrorIs(t, err, injected)
				require.EqualError(t, err, "failed to add movie to library: injected library failure")
			case "later file persistence failure":
				require.EqualError(t, err, "failed to create movie file: injected file failure")
			case "final state failure":
				require.EqualError(t, err, "injected state failure")
			}
			got, err := store.GetMovie(ctx, id)
			require.NoError(t, err)
			require.Equal(t, before, got)
			require.Equal(t, history, movieFollowupHistory(t, db, id))
			// Earlier per-file inserts are committed, not rolled back with later work.
			// Strict ordered library expectations also forbid processing part3 after a
			// part2 failure. This does not pretend the external library is transactional.
			files, err := store.ListMovieFiles(ctx)
			require.NoError(t, err)
			persisted := 1
			if scenario == "final state failure" {
				persisted = 3
			}
			require.Len(t, files, persisted)
			for i, file := range files {
				part := strconv.Itoa(i + 1)
				require.Positive(t, file.ID)
				require.False(t, file.DateAdded.IsZero())
				require.Equal(t, sqlcdb.MovieFile{ID: file.ID, DateAdded: file.DateAdded, RelativePath: sql.NullString{String: "my-movie/part" + part + ".mkv", Valid: true}, OriginalFilePath: sql.NullString{String: "/downloads/part" + part + ".mkv", Valid: true}, Size: int64((i + 1) * 1024)}, file)
			}
		})
	}
}
