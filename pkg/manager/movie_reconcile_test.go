package manager

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/kasuboski/mediaz/config"
	"github.com/kasuboski/mediaz/pkg/download"
	downloadMock "github.com/kasuboski/mediaz/pkg/download/mocks"
	mhttpMock "github.com/kasuboski/mediaz/pkg/http/mocks"
	"github.com/kasuboski/mediaz/pkg/indexer"
	indexerMock "github.com/kasuboski/mediaz/pkg/indexer/mocks"
	"github.com/kasuboski/mediaz/pkg/library"
	mockLibrary "github.com/kasuboski/mediaz/pkg/library/mocks"
	"github.com/kasuboski/mediaz/pkg/prowlarr"
	"github.com/kasuboski/mediaz/pkg/ptr"
	"github.com/kasuboski/mediaz/pkg/storage"
	storageMocks "github.com/kasuboski/mediaz/pkg/storage/mocks"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	mediaSqlite "github.com/kasuboski/mediaz/pkg/storage/sqlite"
	"github.com/kasuboski/mediaz/pkg/tmdb"
	"github.com/oapi-codegen/nullable"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func newMovieReconcileStore(t *testing.T, ctx context.Context) storage.Storage {
	t.Helper()
	store, err := mediaSqlite.New(ctx, ":memory:")
	require.NoError(t, err)
	return store
}

func Test_Manager_reconcileMissingMovie(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	store := newMovieReconcileStore(t, ctx)

	makeRelease := func(id, seeders int32, size int64, title string, protocol prowlarr.DownloadProtocol) *prowlarr.ReleaseResource {
		return &prowlarr.ReleaseResource{ID: &id, Title: nullable.NewNullableWithValue(title), Size: &size, Seeders: nullable.NewNullableWithValue(seeders), Protocol: &protocol}
	}
	wantRelease := makeRelease(123, 23, 23*1024*1024*1024, "translated movie title", prowlarr.DownloadProtocolTorrent)
	wantRelease.ImdbID = ptr.To(int32(123456))
	conflictingRelease := makeRelease(126, 99, 23*1024*1024*1024, "test movie", prowlarr.DownloadProtocolTorrent)
	conflictingRelease.ImdbID = ptr.To(int32(654321))
	releases := []*prowlarr.ReleaseResource{
		conflictingRelease,
		makeRelease(124, 15, 23*1024*1024*1024, "test movie", prowlarr.DownloadProtocolTorrent),
		wantRelease,
		makeRelease(125, 10, 1024*1024*1024, "test movie - very small", prowlarr.DownloadProtocolTorrent),
		makeRelease(1225, 10, 23*1024*1024*1024, "test movie - nzb", prowlarr.DownloadProtocolUsenet),
	}
	source := indexerMock.NewMockIndexerSource(ctrl)
	source.EXPECT().Search(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(releases, nil).Times(2)
	source.EXPECT().ListIndexers(ctx).Return([]indexer.SourceIndexer{{ID: 1, Name: "test", Priority: 1}, {ID: 3, Name: "test2", Priority: 10}}, nil)
	indexerFactory := indexerMock.NewMockFactory(ctrl)
	indexerFactory.EXPECT().NewIndexerSource(gomock.Any()).Return(source, nil).AnyTimes()
	sourceID, err := store.CreateIndexerSource(ctx, sqlcdb.IndexerSource{Name: "test-source", Implementation: "prowlarr", Scheme: "http", Host: "test", Enabled: true})
	require.NoError(t, err)

	client := sqlcdb.DownloadClient{Implementation: "transmission", Type: "torrent", Port: 8080, Host: "transmission", Scheme: "http"}
	client.ID, err = store.CreateDownloadClient(ctx, client)
	require.NoError(t, err)
	factory := downloadMock.NewMockFactory(ctrl)
	dc := downloadMock.NewMockDownloadClient(ctrl)
	factory.EXPECT().NewDownloadClient(client).Return(dc, nil)
	dc.EXPECT().Add(ctx, download.AddRequest{Release: wantRelease}).Return(download.Status{ID: "123", Name: "test download"}, nil)

	metadataID, err := store.CreateMovieMetadata(ctx, sqlcdb.MovieMetadatum{TmdbID: 1234, ImdbID: sql.NullString{String: "tt0123456", Valid: true}, Title: "test movie", Runtime: 120})
	require.NoError(t, err)
	movieID, err := store.CreateMovie(ctx, sqlcdb.Movie{Monitored: 1, QualityProfileID: 1, Path: sql.NullString{String: "test movie", Valid: true}, MovieMetadataID: sql.NullInt64{Int64: metadataID, Valid: true}}, storage.MovieStateMissing)
	require.NoError(t, err)
	movie, err := store.GetMovie(ctx, movieID)
	require.NoError(t, err)
	m := New(nil, indexerFactory, nil, store, factory, config.Manager{}, config.Config{})
	require.NoError(t, m.RefreshIndexerSource(ctx, sourceID))
	indexers, err := m.listIndexersInternal(ctx)
	require.NoError(t, err)
	require.NoError(t, m.reconcileMissingMovie(ctx, sqlcdb.ListMoviesByStatesRow(movie), newReconcileSnapshot(indexers, []sqlcdb.DownloadClient{client})))
	got, err := store.GetMovie(ctx, movieID)
	require.NoError(t, err)
	assert.Equal(t, sqlcdb.GetMovieByIDRow{Movie: movie.Movie, State: string(storage.MovieStateDownloading), DownloadID: sql.NullString{String: "123", Valid: true}, DownloadClientID: sql.NullInt64{Int64: client.ID, Valid: true}}, got)
}

func Test_Manager_reconcileDiscoveredMovie(t *testing.T) {
	for _, tt := range []struct {
		name          string
		results       string
		path          string
		hasMetadata   bool
		alreadyLinked bool
		wantLink      bool
	}{
		{name: "single result", results: `[{"id":1234,"title":"test movie"}]`, path: "test movie", wantLink: true},
		{name: "no results", results: `[]`, path: "test movie"},
		{name: "unrelated results", results: `[{"id":1234,"title":"another movie"},{"id":5678,"title":"something else"}]`, path: "test movie"},
		{name: "unique exact title not first", results: `[{"id":1234,"title":"another movie"},{"id":5678,"title":"test movie"}]`, path: "test movie", wantLink: true},
		{name: "same-name results are ambiguous", results: `[{"id":1234,"title":"test movie"},{"id":5678,"title":"test movie"}]`, path: "test movie"},
		{name: "nil and invalid IDs", results: `[{"id":0,"title":"test movie"},{"title":"test movie"}]`, path: "test movie"},
		{name: "already linked", results: `[{"id":1234,"title":"test movie"}]`, path: "test movie", alreadyLinked: true},
		{name: "has metadata", path: "test movie", hasMetadata: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			ctrl := gomock.NewController(t)
			store := storageMocks.NewMockStorage(ctrl)
			httpClient := mhttpMock.NewMockHTTPClient(ctrl)
			client, err := tmdb.New("https://api.themoviedb.org", "1234", tmdb.WithHTTPClient(httpClient))
			require.NoError(t, err)
			m := New(client, nil, nil, store, nil, config.Manager{}, config.Config{})
			movie := sqlcdb.ListMoviesByStatesRow{Movie: sqlcdb.Movie{ID: 1 << 40, Path: sql.NullString{String: tt.path, Valid: true}}}
			if tt.hasMetadata {
				movie.Movie.MovieMetadataID = sql.NullInt64{Int64: 120, Valid: true}
			} else {
				httpClient.EXPECT().Do(gomock.Any()).Return(&http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"results":` + tt.results + `}`))}, nil)
				if tt.wantLink || tt.alreadyLinked {
					metadataTmdbID := int64(1234)
					if tt.name == "unique exact title not first" {
						metadataTmdbID = 5678
					}
					store.EXPECT().GetMovieMetadataByTmdbID(ctx, metadataTmdbID).Return(sqlcdb.MovieMetadatum{ID: 120}, nil)
					if tt.alreadyLinked {
						store.EXPECT().GetMovieByMetadataID(ctx, int64(120)).Return(sqlcdb.GetMovieByMetadataIDRow{Movie: sqlcdb.Movie{ID: 1}}, nil)
					} else {
						store.EXPECT().GetMovieByMetadataID(ctx, int64(120)).Return(sqlcdb.GetMovieByMetadataIDRow{}, storage.ErrNotFound)
						store.EXPECT().LinkMovieMetadata(ctx, movie.Movie.ID, int64(120)).Return(nil)
					}
				}
			}
			before := movie
			require.NoError(t, m.reconcileDiscoveredMovie(ctx, movie))
			// The generated read row is a value snapshot; linking is verified at the storage seam.
			assert.Equal(t, before, movie)
		})
	}
}

func Test_Manager_reconcileUnreleasedMovie(t *testing.T) {
	ctx := context.Background()
	store := newMovieReconcileStore(t, ctx)
	metadataID, err := store.CreateMovieMetadata(ctx, sqlcdb.MovieMetadatum{TmdbID: 1234, Title: "test movie", ReleaseDate: sql.NullTime{Time: time.Now().AddDate(0, 0, 5), Valid: true}})
	require.NoError(t, err)
	movieID, err := store.CreateMovie(ctx, sqlcdb.Movie{Monitored: 1, QualityProfileID: 1, MovieMetadataID: sql.NullInt64{Int64: metadataID, Valid: true}}, storage.MovieStateUnreleased)
	require.NoError(t, err)
	movie, err := store.GetMovie(ctx, movieID)
	require.NoError(t, err)
	m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
	require.NoError(t, m.reconcileUnreleasedMovie(ctx, sqlcdb.ListMoviesByStatesRow(movie), newReconcileSnapshot(nil, nil)))
	got, err := store.GetMovie(ctx, movieID)
	require.NoError(t, err)
	assert.Equal(t, movie, got)
}

func Test_Manager_reconcileDownloadingMovie(t *testing.T) {
	ctx := context.Background()
	t.Run("snapshot is nil", func(t *testing.T) {
		m := New(nil, nil, nil, nil, nil, config.Manager{}, config.Config{})
		require.NoError(t, m.reconcileDownloadingMovie(ctx, sqlcdb.ListMoviesByStatesRow{Movie: sqlcdb.Movie{ID: 1}}, nil))
	})
	t.Run("movie is not monitored", func(t *testing.T) {
		m := New(nil, nil, nil, nil, nil, config.Manager{}, config.Config{})
		require.NoError(t, m.reconcileDownloadingMovie(ctx, sqlcdb.ListMoviesByStatesRow{Movie: sqlcdb.Movie{ID: 1}}, newReconcileSnapshot(nil, nil)))
	})
	t.Run("movie file is already tracked", func(t *testing.T) {
		store := newMovieReconcileStore(t, ctx)
		movieID, err := store.CreateMovie(ctx, sqlcdb.Movie{Monitored: 1, QualityProfileID: 1, Path: sql.NullString{String: "Movie 1", Valid: true}}, storage.MovieStateMissing)
		require.NoError(t, err)
		require.NoError(t, store.UpdateMovieState(ctx, movieID, storage.MovieStateDownloading, nil))
		_, err = store.CreateMovieFile(ctx, sqlcdb.MovieFile{RelativePath: sql.NullString{String: "Movie 1/movie1.mp4", Valid: true}})
		require.NoError(t, err)
		movie, err := store.GetMovie(ctx, movieID)
		require.NoError(t, err)
		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		require.NoError(t, m.reconcileDownloadingMovie(ctx, sqlcdb.ListMoviesByStatesRow(movie), newReconcileSnapshot(nil, nil)))
		got, err := store.GetMovie(ctx, movieID)
		require.NoError(t, err)
		assert.Equal(t, sqlcdb.GetMovieByIDRow{Movie: movie.Movie, State: string(storage.MovieStateDownloaded)}, got)
	})
	t.Run("download client not found in snapshot", func(t *testing.T) {
		store := newMovieReconcileStore(t, ctx)
		movieID, err := store.CreateMovie(ctx, sqlcdb.Movie{Monitored: 1, QualityProfileID: 1, Path: sql.NullString{String: "Movie 1", Valid: true}}, storage.MovieStateMissing)
		require.NoError(t, err)
		require.NoError(t, store.UpdateMovieState(ctx, movieID, storage.MovieStateDownloading, nil))
		movie, err := store.GetMovie(ctx, movieID)
		require.NoError(t, err)
		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		require.NoError(t, m.reconcileDownloadingMovie(ctx, sqlcdb.ListMoviesByStatesRow(movie), newReconcileSnapshot(nil, nil)))
		got, err := store.GetMovie(ctx, movieID)
		require.NoError(t, err)
		assert.Equal(t, movie, got)
	})

	for _, tt := range []struct {
		name         string
		factoryError error
		getError     error
		done         bool
		libraryError error
		wantError    string
		wantState    storage.MovieState
	}{
		{name: "failed to create download client from model", factoryError: errors.New("failed to create download client"), wantError: "failed to create download client", wantState: storage.MovieStateDownloading},
		{name: "failed to get download status", getError: errors.New("failed to get download status"), wantError: "failed to get download status", wantState: storage.MovieStateDownloading},
		{name: "download not finished", wantState: storage.MovieStateDownloading},
		{name: "failed to add movie file to library", done: true, libraryError: errors.New("expected testing error"), wantError: "failed to add movie to library: expected testing error", wantState: storage.MovieStateDownloading},
		{name: "successfully reconciled downloading movie", done: true, wantState: storage.MovieStateDownloaded},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			store := newMovieReconcileStore(t, ctx)
			metadataID, err := store.CreateMovieMetadata(ctx, sqlcdb.MovieMetadatum{Title: "my-movie", TmdbID: 1234})
			require.NoError(t, err)
			client := sqlcdb.DownloadClient{Implementation: "transmission", Type: "torrent", Port: 8080, Host: "transmission", Scheme: "http"}
			client.ID, err = store.CreateDownloadClient(ctx, client)
			require.NoError(t, err)
			movieID, err := store.CreateMovie(ctx, sqlcdb.Movie{Monitored: 1, QualityProfileID: 1, MovieMetadataID: sql.NullInt64{Int64: metadataID, Valid: true}, Path: sql.NullString{String: "my-movie", Valid: true}}, storage.MovieStateMissing)
			require.NoError(t, err)
			require.NoError(t, store.UpdateMovieState(ctx, movieID, storage.MovieStateDownloading, &storage.TransitionStateMetadata{DownloadID: ptr.To("123"), DownloadClientID: &client.ID}))
			movie, err := store.GetMovie(ctx, movieID)
			require.NoError(t, err)
			factory := downloadMock.NewMockFactory(ctrl)
			dc := downloadMock.NewMockDownloadClient(ctrl)
			if tt.factoryError != nil {
				factory.EXPECT().NewDownloadClient(client).Return(nil, tt.factoryError)
			} else {
				factory.EXPECT().NewDownloadClient(client).Return(dc, nil)
				dc.EXPECT().Get(ctx, download.GetRequest{ID: "123"}).Return(download.Status{ID: "123", Done: tt.done, FilePaths: []string{"/downloads/movie.mp4"}}, tt.getError)
			}
			lib := mockLibrary.NewMockLibrary(ctrl)
			if tt.done {
				lib.EXPECT().AddMovie(ctx, "my-movie", "/downloads/movie.mp4").Return(library.MovieFile{Name: "my-movie", RelativePath: "my-movie/movie.mp4", AbsolutePath: "/movies/my-movie/movie.mp4", Size: 1024}, tt.libraryError)
			}
			m := New(nil, nil, lib, store, factory, config.Manager{}, config.Config{})
			err = m.reconcileDownloadingMovie(ctx, sqlcdb.ListMoviesByStatesRow(movie), newReconcileSnapshot(nil, []sqlcdb.DownloadClient{client}))
			if tt.wantError != "" {
				require.EqualError(t, err, tt.wantError)
			} else {
				require.NoError(t, err)
			}
			got, err := store.GetMovie(ctx, movieID)
			require.NoError(t, err)
			want := movie
			if tt.wantState == storage.MovieStateDownloaded {
				want.State = string(tt.wantState)
				want.DownloadID = sql.NullString{}
				want.DownloadClientID = sql.NullInt64{}
			}
			assert.Equal(t, want, got)
			if tt.wantState == storage.MovieStateDownloaded {
				files, err := store.GetMovieFilesByMovieName(ctx, "my-movie")
				require.NoError(t, err)
				require.Len(t, files, 1)
				assert.Equal(t, sqlcdb.MovieFile{ID: files[0].ID, DateAdded: files[0].DateAdded, RelativePath: sql.NullString{String: "my-movie/movie.mp4", Valid: true}, OriginalFilePath: sql.NullString{String: "/downloads/movie.mp4", Valid: true}, Size: 1024}, files[0])
			}
		})
	}

	t.Run("failed to get movie metadata", func(t *testing.T) {
		// A missing referenced metadata row cannot be created with foreign keys on.
		ctrl := gomock.NewController(t)
		store := storageMocks.NewMockStorage(ctrl)
		movie := sqlcdb.ListMoviesByStatesRow{Movie: sqlcdb.Movie{ID: 1, Monitored: 1, Path: sql.NullString{String: "Movie 1", Valid: true}, MovieMetadataID: sql.NullInt64{Int64: 99, Valid: true}}, State: string(storage.MovieStateDownloading), DownloadClientID: sql.NullInt64{Int64: 1, Valid: true}, DownloadID: sql.NullString{String: "123", Valid: true}}
		store.EXPECT().GetMovieFilesByMovieName(ctx, "Movie 1").Return(nil, storage.ErrNotFound)
		store.EXPECT().GetMovieMetadataByID(ctx, int64(99)).Return(sqlcdb.MovieMetadatum{}, storage.ErrNotFound)
		client := sqlcdb.DownloadClient{ID: 1, Implementation: "transmission", Type: "torrent"}
		factory := downloadMock.NewMockFactory(ctrl)
		dc := downloadMock.NewMockDownloadClient(ctrl)
		factory.EXPECT().NewDownloadClient(client).Return(dc, nil)
		dc.EXPECT().Get(ctx, download.GetRequest{ID: "123"}).Return(download.Status{ID: "123", Done: true}, nil)
		m := New(nil, nil, nil, store, factory, config.Manager{}, config.Config{})
		require.ErrorIs(t, m.reconcileDownloadingMovie(ctx, movie, newReconcileSnapshot(nil, []sqlcdb.DownloadClient{client})), storage.ErrNotFound)
	})
}

func Test_Manager_reconcileMissingMovie_MovieFileIDAlreadySet(t *testing.T) {
	ctx := context.Background()
	store := newMovieReconcileStore(t, ctx)
	metadataID, err := store.CreateMovieMetadata(ctx, sqlcdb.MovieMetadatum{Title: "test-movie", TmdbID: 1234})
	require.NoError(t, err)
	movieID, err := store.CreateMovie(ctx, sqlcdb.Movie{Monitored: 1, QualityProfileID: 1, Path: sql.NullString{String: "test-movie", Valid: true}, MovieMetadataID: sql.NullInt64{Int64: metadataID, Valid: true}}, storage.MovieStateMissing)
	require.NoError(t, err)
	fileID, err := store.CreateMovieFile(ctx, sqlcdb.MovieFile{RelativePath: sql.NullString{String: "test-movie/test-movie.mkv", Valid: true}, Size: 2048, OriginalFilePath: sql.NullString{String: "/downloads/test-movie.mkv", Valid: true}})
	require.NoError(t, err)
	require.NoError(t, store.UpdateMovieMovieFileID(ctx, movieID, fileID))
	movie, err := store.GetMovie(ctx, movieID)
	require.NoError(t, err)
	m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
	require.NoError(t, m.reconcileMissingMovie(ctx, sqlcdb.ListMoviesByStatesRow(movie), newReconcileSnapshot(nil, nil)))
	got, err := store.GetMovie(ctx, movieID)
	require.NoError(t, err)
	assert.Equal(t, sqlcdb.GetMovieByIDRow{Movie: movie.Movie, State: string(storage.MovieStateDownloaded)}, got)
}

func Test_Manager_ReconcileMovies(t *testing.T) {
	t.Run("returns nil when all sub-reconcilers succeed", func(t *testing.T) {
		ctx := context.Background()
		m := New(nil, nil, nil, newMovieReconcileStore(t, ctx), nil, config.Manager{}, config.Config{})
		require.NoError(t, m.ReconcileMovies(ctx))
	})
	t.Run("returns error when ListDownloadClients fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		ctx := context.Background()
		store := storageMocks.NewMockStorage(ctrl)
		store.EXPECT().ListDownloadClients(ctx).Return(nil, errors.New("download client error"))
		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		require.EqualError(t, m.ReconcileMovies(ctx), "download client error")
	})
	t.Run("aggregates errors from multiple sub-reconcilers", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		ctx := context.Background()
		store := storageMocks.NewMockStorage(ctrl)
		store.EXPECT().ListDownloadClients(ctx).Return(nil, nil)
		store.EXPECT().ListIndexers(gomock.Any()).Return(nil, errors.New("indexer error"))
		store.EXPECT().ListMoviesByState(gomock.Any(), storage.MovieStateUnreleased).Return(nil, errors.New("unreleased error"))
		store.EXPECT().ListMoviesByState(gomock.Any(), storage.MovieStateDownloading).Return(nil, nil)
		store.EXPECT().ListMoviesByState(gomock.Any(), storage.MovieStateDiscovered).Return(nil, nil)
		m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})
		err := m.ReconcileMovies(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "indexer error")
		assert.Contains(t, err.Error(), "unreleased error")
	})
}
