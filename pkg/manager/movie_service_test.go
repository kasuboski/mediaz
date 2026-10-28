package manager

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

	mockLibrary "github.com/kasuboski/mediaz/pkg/library/mocks"
	"github.com/kasuboski/mediaz/pkg/pagination"
	"github.com/kasuboski/mediaz/pkg/ptr"
	"github.com/kasuboski/mediaz/pkg/storage"
	storageMocks "github.com/kasuboski/mediaz/pkg/storage/mocks"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/kasuboski/mediaz/pkg/tmdb"
	tmdbMocks "github.com/kasuboski/mediaz/pkg/tmdb/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// mockMovieMetadataProvider is a test double for MovieMetadataProvider
type mockMovieMetadataProvider struct {
	metadata    *sqlcdb.MovieMetadatum
	metadataMap map[int64]*sqlcdb.MovieMetadatum
	err         error
}

func (m *mockMovieMetadataProvider) GetMovieMetadata(ctx context.Context, tmdbID int64) (*sqlcdb.MovieMetadatum, error) {
	return m.metadata, m.err
}

func (m *mockMovieMetadataProvider) GetMovieMetadataByID(ctx context.Context, metadataID int64) (*sqlcdb.MovieMetadatum, error) {
	if m.metadataMap != nil {
		if meta, ok := m.metadataMap[metadataID]; ok {
			return meta, nil
		}
	}
	return m.metadata, m.err
}

func newTestMovieService(ctrl *gomock.Controller) (*MovieService, *storageMocks.MockStorage, *tmdbMocks.MockITmdb, *mockLibrary.MockLibrary) {
	store := storageMocks.NewMockStorage(ctrl)
	tmdbClient := tmdbMocks.NewMockITmdb(ctrl)
	lib := mockLibrary.NewMockLibrary(ctrl)
	qualityService := NewQualityService(store)
	metadataProvider := &mockMovieMetadataProvider{}

	svc := NewMovieService(tmdbClient, lib, store, qualityService, metadataProvider)
	return svc, store, tmdbClient, lib
}

func TestNewMovieService(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := storageMocks.NewMockStorage(ctrl)
	tmdbClient := tmdbMocks.NewMockITmdb(ctrl)
	lib := mockLibrary.NewMockLibrary(ctrl)
	qualityService := NewQualityService(store)
	metadataProvider := &mockMovieMetadataProvider{}

	svc := NewMovieService(tmdbClient, lib, store, qualityService, metadataProvider)
	require.NotNil(t, svc)
	assert.Equal(t, tmdbClient, svc.tmdb)
	assert.Equal(t, lib, svc.library)
	assert.NotNil(t, svc.qualityService)
	assert.Equal(t, metadataProvider, svc.metadataProvider)
}

// ---------------------------------------------------------------------------
// CRUD Tests
// ---------------------------------------------------------------------------

func TestMovieService_AddMovieToLibrary(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	svc, store, _, _ := newTestMovieService(ctrl)

	releaseDate := time.Now().AddDate(0, 0, -1)
	metadata := &sqlcdb.MovieMetadatum{
		ID:          1,
		TmdbID:      1234,
		Title:       "Test Movie",
		ReleaseDate: sql.NullTime{Time: releaseDate, Valid: true},
	}

	svc.metadataProvider = &mockMovieMetadataProvider{metadata: metadata}

	store.EXPECT().GetQualityProfile(ctx, int64(1)).Return(sqlcdb.QualityProfile{ID: 1}, nil)
	store.EXPECT().GetMovieByMetadataID(ctx, int64(1)).Return(sqlcdb.GetMovieByMetadataIDRow{}, storage.ErrNotFound)
	store.EXPECT().CreateMovie(ctx, gomock.Any(), storage.MovieStateMissing).Return(int64(1), nil)
	store.EXPECT().GetMovie(ctx, int64(1)).Return(sqlcdb.GetMovieByIDRow{
		Movie: sqlcdb.Movie{
			ID:               1,
			Monitored:        1,
			QualityProfileID: 1,
			MovieMetadataID:  sql.NullInt64{Int64: 1, Valid: true},
			Path:             sql.NullString{String: "Test Movie", Valid: true},
		},
		State: string(storage.MovieStateMissing),
	}, nil)

	movie, err := svc.AddMovieToLibrary(ctx, AddMovieRequest{TMDBID: 1234, QualityProfileID: 1})
	require.NoError(t, err)
	require.NotNil(t, movie)

	expected := &sqlcdb.GetMovieByIDRow{
		Movie: sqlcdb.Movie{
			ID:               1,
			Monitored:        1,
			QualityProfileID: 1,
			MovieMetadataID:  sql.NullInt64{Int64: 1, Valid: true},
			Path:             sql.NullString{String: "Test Movie", Valid: true},
		},
		State: string(storage.MovieStateMissing),
	}
	assert.Equal(t, expected, movie)
}

func TestMovieService_AddMovieToLibrary_AlreadyExists_Downloaded(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	svc, store, _, _ := newTestMovieService(ctrl)

	metadata := &sqlcdb.MovieMetadatum{
		ID:     1,
		TmdbID: 1234,
		Title:  "Existing Movie",
	}

	svc.metadataProvider = &mockMovieMetadataProvider{metadata: metadata}

	store.EXPECT().GetQualityProfile(ctx, int64(1)).Return(sqlcdb.QualityProfile{ID: 1}, nil)
	store.EXPECT().GetMovieByMetadataID(ctx, int64(1)).Return(sqlcdb.GetMovieByMetadataIDRow{
		Movie: sqlcdb.Movie{
			ID:               5,
			MovieMetadataID:  sql.NullInt64{Int64: 1, Valid: true},
			Monitored:        1,
			QualityProfileID: 1,
		},
		State: string(storage.MovieStateDownloaded),
	}, nil)

	movie, err := svc.AddMovieToLibrary(ctx, AddMovieRequest{TMDBID: 1234, QualityProfileID: 1})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrMovieAlreadyExists))
	require.NotNil(t, movie)

	expected := &sqlcdb.GetMovieByIDRow{
		Movie: sqlcdb.Movie{
			ID:               5,
			MovieMetadataID:  sql.NullInt64{Int64: 1, Valid: true},
			Monitored:        1,
			QualityProfileID: 1,
		},
		State: string(storage.MovieStateDownloaded),
	}
	assert.Equal(t, expected, movie)
}

func TestMovieService_AddMovieToLibrary_AlreadyExists_Downloading(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	svc, store, _, _ := newTestMovieService(ctrl)

	metadata := &sqlcdb.MovieMetadatum{
		ID:     1,
		TmdbID: 1234,
		Title:  "Downloading Movie",
	}

	svc.metadataProvider = &mockMovieMetadataProvider{metadata: metadata}

	store.EXPECT().GetQualityProfile(ctx, int64(1)).Return(sqlcdb.QualityProfile{ID: 1}, nil)
	store.EXPECT().GetMovieByMetadataID(ctx, int64(1)).Return(sqlcdb.GetMovieByMetadataIDRow{
		Movie: sqlcdb.Movie{
			ID:               6,
			MovieMetadataID:  sql.NullInt64{Int64: 1, Valid: true},
			Monitored:        1,
			QualityProfileID: 1,
		},
		State: string(storage.MovieStateDownloading),
	}, nil)

	movie, err := svc.AddMovieToLibrary(ctx, AddMovieRequest{TMDBID: 1234, QualityProfileID: 1})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrMovieAlreadyExists))
	require.NotNil(t, movie)

	expected := &sqlcdb.GetMovieByIDRow{
		Movie: sqlcdb.Movie{
			ID:               6,
			MovieMetadataID:  sql.NullInt64{Int64: 1, Valid: true},
			Monitored:        1,
			QualityProfileID: 1,
		},
		State: string(storage.MovieStateDownloading),
	}
	assert.Equal(t, expected, movie)
}

func TestMovieService_AddMovieToLibrary_AlreadyExists_Discovered(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	svc, store, _, _ := newTestMovieService(ctrl)

	metadata := &sqlcdb.MovieMetadatum{
		ID:     1,
		TmdbID: 1234,
		Title:  "Discovered Movie",
	}

	svc.metadataProvider = &mockMovieMetadataProvider{metadata: metadata}

	store.EXPECT().GetQualityProfile(ctx, int64(1)).Return(sqlcdb.QualityProfile{ID: 1}, nil)
	store.EXPECT().GetMovieByMetadataID(ctx, int64(1)).Return(sqlcdb.GetMovieByMetadataIDRow{
		Movie: sqlcdb.Movie{
			ID:               7,
			MovieMetadataID:  sql.NullInt64{Int64: 1, Valid: true},
			Monitored:        1,
			QualityProfileID: 1,
		},
		State: string(storage.MovieStateDiscovered),
	}, nil)

	movie, err := svc.AddMovieToLibrary(ctx, AddMovieRequest{TMDBID: 1234, QualityProfileID: 1})
	require.NoError(t, err)
	require.NotNil(t, movie)

	expected := &sqlcdb.GetMovieByIDRow{
		Movie: sqlcdb.Movie{
			ID:               7,
			MovieMetadataID:  sql.NullInt64{Int64: 1, Valid: true},
			Monitored:        1,
			QualityProfileID: 1,
		},
		State: string(storage.MovieStateDiscovered),
	}
	assert.Equal(t, expected, movie)
}

func TestMovieService_AddMovieToLibrary_AlreadyExists_Missing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	svc, store, _, _ := newTestMovieService(ctrl)

	metadata := &sqlcdb.MovieMetadatum{
		ID:     1,
		TmdbID: 1234,
		Title:  "Missing Movie",
	}

	svc.metadataProvider = &mockMovieMetadataProvider{metadata: metadata}

	store.EXPECT().GetQualityProfile(ctx, int64(1)).Return(sqlcdb.QualityProfile{ID: 1}, nil)
	store.EXPECT().GetMovieByMetadataID(ctx, int64(1)).Return(sqlcdb.GetMovieByMetadataIDRow{
		Movie: sqlcdb.Movie{
			ID:               8,
			MovieMetadataID:  sql.NullInt64{Int64: 1, Valid: true},
			Monitored:        1,
			QualityProfileID: 1,
		},
		State: string(storage.MovieStateMissing),
	}, nil)

	movie, err := svc.AddMovieToLibrary(ctx, AddMovieRequest{TMDBID: 1234, QualityProfileID: 1})
	require.NoError(t, err)
	require.NotNil(t, movie)

	expected := &sqlcdb.GetMovieByIDRow{
		Movie: sqlcdb.Movie{
			ID:               8,
			MovieMetadataID:  sql.NullInt64{Int64: 1, Valid: true},
			Monitored:        1,
			QualityProfileID: 1,
		},
		State: string(storage.MovieStateMissing),
	}
	assert.Equal(t, expected, movie)
}

func TestMovieService_AddMovieToLibrary_AlreadyExists_Unreleased(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	svc, store, _, _ := newTestMovieService(ctrl)

	metadata := &sqlcdb.MovieMetadatum{
		ID:     1,
		TmdbID: 1234,
		Title:  "Unreleased Movie",
	}

	svc.metadataProvider = &mockMovieMetadataProvider{metadata: metadata}

	store.EXPECT().GetQualityProfile(ctx, int64(1)).Return(sqlcdb.QualityProfile{ID: 1}, nil)
	store.EXPECT().GetMovieByMetadataID(ctx, int64(1)).Return(sqlcdb.GetMovieByMetadataIDRow{
		Movie: sqlcdb.Movie{
			ID:               9,
			MovieMetadataID:  sql.NullInt64{Int64: 1, Valid: true},
			Monitored:        1,
			QualityProfileID: 1,
		},
		State: string(storage.MovieStateUnreleased),
	}, nil)

	movie, err := svc.AddMovieToLibrary(ctx, AddMovieRequest{TMDBID: 1234, QualityProfileID: 1})
	require.NoError(t, err)
	require.NotNil(t, movie)

	expected := &sqlcdb.GetMovieByIDRow{
		Movie: sqlcdb.Movie{
			ID:               9,
			MovieMetadataID:  sql.NullInt64{Int64: 1, Valid: true},
			Monitored:        1,
			QualityProfileID: 1,
		},
		State: string(storage.MovieStateUnreleased),
	}
	assert.Equal(t, expected, movie)
}

func TestMovieService_AddMovieToLibrary_QualityProfileError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	svc, store, _, _ := newTestMovieService(ctrl)

	store.EXPECT().GetQualityProfile(ctx, int64(99)).Return(sqlcdb.QualityProfile{}, errors.New("quality profile not found"))

	movie, err := svc.AddMovieToLibrary(ctx, AddMovieRequest{TMDBID: 1234, QualityProfileID: 99})
	require.Error(t, err)
	assert.Nil(t, movie)
}

func TestMovieService_AddMovieToLibrary_MetadataError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	svc, store, _, _ := newTestMovieService(ctrl)

	svc.metadataProvider = &mockMovieMetadataProvider{err: errors.New("metadata fetch failed")}

	store.EXPECT().GetQualityProfile(ctx, int64(1)).Return(sqlcdb.QualityProfile{ID: 1}, nil)

	movie, err := svc.AddMovieToLibrary(ctx, AddMovieRequest{TMDBID: 1234, QualityProfileID: 1})
	require.Error(t, err)
	assert.Nil(t, movie)
}

func TestMovieService_AddMovieToLibrary_Unreleased(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	svc, store, _, _ := newTestMovieService(ctrl)

	// Future release date → state should be Unreleased
	releaseDate := time.Now().AddDate(1, 0, 0)
	metadata := &sqlcdb.MovieMetadatum{
		ID:          1,
		TmdbID:      1234,
		Title:       "Future Movie",
		ReleaseDate: sql.NullTime{Time: releaseDate, Valid: true},
	}

	svc.metadataProvider = &mockMovieMetadataProvider{metadata: metadata}

	store.EXPECT().GetQualityProfile(ctx, int64(1)).Return(sqlcdb.QualityProfile{ID: 1}, nil)
	store.EXPECT().GetMovieByMetadataID(ctx, int64(1)).Return(sqlcdb.GetMovieByMetadataIDRow{}, storage.ErrNotFound)
	store.EXPECT().CreateMovie(ctx, gomock.Any(), storage.MovieStateUnreleased).Return(int64(1), nil)
	store.EXPECT().GetMovie(ctx, int64(1)).Return(sqlcdb.GetMovieByIDRow{
		Movie: sqlcdb.Movie{
			ID:               1,
			Monitored:        1,
			QualityProfileID: 1,
			MovieMetadataID:  sql.NullInt64{Int64: 1, Valid: true},
			Path:             sql.NullString{String: "Future Movie", Valid: true},
		},
		State: string(storage.MovieStateUnreleased),
	}, nil)

	movie, err := svc.AddMovieToLibrary(ctx, AddMovieRequest{TMDBID: 1234, QualityProfileID: 1})
	require.NoError(t, err)
	require.NotNil(t, movie)

	expected := &sqlcdb.GetMovieByIDRow{
		Movie: sqlcdb.Movie{
			ID:               1,
			Monitored:        1,
			QualityProfileID: 1,
			MovieMetadataID:  sql.NullInt64{Int64: 1, Valid: true},
			Path:             sql.NullString{String: "Future Movie", Valid: true},
		},
		State: string(storage.MovieStateUnreleased),
	}
	assert.Equal(t, expected, movie)
}

func TestMovieService_AddMovieToLibrary_CreateError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	svc, store, _, _ := newTestMovieService(ctrl)

	releaseDate := time.Now().AddDate(0, 0, -1)
	metadata := &sqlcdb.MovieMetadatum{
		ID:          1,
		TmdbID:      1234,
		Title:       "Test Movie",
		ReleaseDate: sql.NullTime{Time: releaseDate, Valid: true},
	}

	svc.metadataProvider = &mockMovieMetadataProvider{metadata: metadata}

	store.EXPECT().GetQualityProfile(ctx, int64(1)).Return(sqlcdb.QualityProfile{ID: 1}, nil)
	store.EXPECT().GetMovieByMetadataID(ctx, int64(1)).Return(sqlcdb.GetMovieByMetadataIDRow{}, storage.ErrNotFound)
	store.EXPECT().CreateMovie(ctx, gomock.Any(), gomock.Any()).Return(int64(0), errors.New("db insert failed"))

	movie, err := svc.AddMovieToLibrary(ctx, AddMovieRequest{TMDBID: 1234, QualityProfileID: 1})
	require.Error(t, err)
	assert.Nil(t, movie)
}

func TestMovieService_UpdateMovieMonitored(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	svc, store, _, _ := newTestMovieService(ctrl)

	movie := &sqlcdb.GetMovieByIDRow{
		Movie: sqlcdb.Movie{
			ID:        1,
			Monitored: 1,
		},
	}

	store.EXPECT().UpdateMovieByID(ctx, gomock.Any(), gomock.Any()).Return(nil)
	store.EXPECT().GetMovie(ctx, int64(1)).Return(*movie, nil)

	result, err := svc.UpdateMovieMonitored(ctx, 1, true)
	require.NoError(t, err)
	require.NotNil(t, result)

	expected := &sqlcdb.GetMovieByIDRow{
		Movie: sqlcdb.Movie{
			ID:        1,
			Monitored: 1,
		},
	}
	assert.Equal(t, expected, result)
}

func TestMovieService_UpdateMovieMonitored_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	svc, store, _, _ := newTestMovieService(ctrl)

	store.EXPECT().UpdateMovieByID(ctx, gomock.Any(), gomock.Any()).Return(errors.New("db error"))

	result, err := svc.UpdateMovieMonitored(ctx, 1, true)
	require.Error(t, err)
	assert.Nil(t, result)
}

func TestMovieService_UpdateMovieQualityProfile(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	svc, store, _, _ := newTestMovieService(ctrl)

	movie := &sqlcdb.GetMovieByIDRow{
		Movie: sqlcdb.Movie{
			ID:               1,
			QualityProfileID: 3,
		},
	}

	store.EXPECT().UpdateMovieQualityProfile(ctx, int64(1), int64(3)).Return(nil)
	store.EXPECT().GetMovie(ctx, int64(1)).Return(*movie, nil)

	result, err := svc.UpdateMovieQualityProfile(ctx, 1, 3)
	require.NoError(t, err)
	require.NotNil(t, result)

	expected := &sqlcdb.GetMovieByIDRow{
		Movie: sqlcdb.Movie{
			ID:               1,
			QualityProfileID: 3,
		},
	}
	assert.Equal(t, expected, result)
}

func TestMovieService_UpdateMovieQualityProfile_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	svc, store, _, _ := newTestMovieService(ctrl)

	store.EXPECT().UpdateMovieQualityProfile(ctx, int64(1), int64(3)).Return(errors.New("db error"))

	result, err := svc.UpdateMovieQualityProfile(ctx, 1, 3)
	require.Error(t, err)
	assert.Nil(t, result)
}

func TestMovieService_DeleteMovie(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	svc, store, _, _ := newTestMovieService(ctrl)

	path := "Test Movie"
	movie := &sqlcdb.GetMovieByIDRow{
		Movie: sqlcdb.Movie{
			ID:   1,
			Path: sql.NullString{String: path, Valid: true},
		},
	}

	store.EXPECT().GetMovie(ctx, int64(1)).Return(*movie, nil)
	store.EXPECT().DeleteMovie(ctx, int64(1)).Return(nil)

	err := svc.DeleteMovie(ctx, 1, false)
	require.NoError(t, err)
}

func TestMovieService_DeleteMovie_WithFiles(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	svc, store, _, lib := newTestMovieService(ctrl)

	path := "Test Movie"
	movie := &sqlcdb.GetMovieByIDRow{
		Movie: sqlcdb.Movie{
			ID:   1,
			Path: sql.NullString{String: path, Valid: true},
		},
	}

	store.EXPECT().GetMovie(ctx, int64(1)).Return(*movie, nil)
	lib.EXPECT().DeleteMovieDirectory(ctx, path).Return(nil)
	store.EXPECT().DeleteMovie(ctx, int64(1)).Return(nil)

	err := svc.DeleteMovie(ctx, 1, true)
	require.NoError(t, err)
}

func TestMovieService_DeleteMovie_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	svc, store, _, _ := newTestMovieService(ctrl)

	store.EXPECT().GetMovie(ctx, int64(1)).Return(sqlcdb.GetMovieByIDRow{}, storage.ErrNotFound)

	err := svc.DeleteMovie(ctx, 1, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get movie")
}

func TestMovieService_DeleteMovie_NilPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	svc, store, _, _ := newTestMovieService(ctrl)

	movie := &sqlcdb.GetMovieByIDRow{
		Movie: sqlcdb.Movie{
			ID:   1,
			Path: sql.NullString{},
		},
	}

	store.EXPECT().GetMovie(ctx, int64(1)).Return(*movie, nil)

	err := svc.DeleteMovie(ctx, 1, true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "movie path is nil")
}

// ---------------------------------------------------------------------------
// Listing Tests
// ---------------------------------------------------------------------------

func TestMovieService_ListMoviesInLibrary(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	svc, store, _, _ := newTestMovieService(ctrl)

	path := "movie1"
	year := int64(2024)
	movieMetadata := sqlcdb.MovieMetadatum{TmdbID: 123, Title: "Test Movie", Images: "poster.jpg", Year: sql.NullInt64{Int64: year, Valid: true}}
	rows := []sqlcdb.ListLibraryMoviesRow{
		{Movie: sqlcdb.Movie{Path: sql.NullString{String: path, Valid: true}}, MovieMetadatum: movieMetadata, State: string(storage.MovieStateDiscovered)},
		{Movie: sqlcdb.Movie{Path: sql.NullString{String: path, Valid: true}}, MovieMetadatum: movieMetadata, State: string(storage.MovieStateDownloaded)},
	}
	params := pagination.Params{Page: 1, PageSize: 50}
	store.EXPECT().ListLibraryMovies(ctx, 0, 50, []storage.MovieState(nil)).Return(rows, nil)
	store.EXPECT().CountLibraryMovies(ctx, []storage.MovieState(nil)).Return(int64(2), nil)
	store.EXPECT().CountLibraryMoviesByState(ctx).Return([]sqlcdb.CountLibraryMoviesByStateRow{{State: "discovered", Count: 1}, {State: "downloaded", Count: 1}}, nil)

	response, err := svc.ListMoviesInLibrary(ctx, params, nil)
	require.NoError(t, err)
	assert.Equal(t, LibraryMoviesResponse{
		Items: []LibraryMovie{
			{Path: path, TMDBID: 123, Title: "Test Movie", PosterPath: "poster.jpg", Year: 2024, State: "discovered"},
			{Path: path, TMDBID: 123, Title: "Test Movie", PosterPath: "poster.jpg", Year: 2024, State: "downloaded"},
		},
		Pagination: pagination.Meta{Page: 1, PageSize: 50, TotalItems: 2, TotalPages: 1},
		ByState:    map[string]int64{"discovered": 1, "downloaded": 1},
	}, response)
}

func TestMovieService_ListMoviesInLibrary_Empty(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	svc, store, _, _ := newTestMovieService(ctrl)

	params := pagination.Params{Page: 1, PageSize: 50}
	store.EXPECT().ListLibraryMovies(ctx, 0, 50, []storage.MovieState(nil)).Return([]sqlcdb.ListLibraryMoviesRow{}, nil)
	store.EXPECT().CountLibraryMovies(ctx, []storage.MovieState(nil)).Return(int64(0), nil)
	store.EXPECT().CountLibraryMoviesByState(ctx).Return([]sqlcdb.CountLibraryMoviesByStateRow{}, nil)

	response, err := svc.ListMoviesInLibrary(ctx, params, nil)
	require.NoError(t, err)
	assert.Equal(t, LibraryMoviesResponse{Items: []LibraryMovie{}, Pagination: pagination.Meta{Page: 1, PageSize: 50}, ByState: map[string]int64{}}, response)
}

func TestMovieService_ListMoviesInLibrary_SkipsNoMetadata(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	svc, store, _, _ := newTestMovieService(ctrl)

	store.EXPECT().ListLibraryMovies(ctx, 0, 50, []storage.MovieState(nil)).Return([]sqlcdb.ListLibraryMoviesRow{}, nil)
	store.EXPECT().CountLibraryMovies(ctx, []storage.MovieState(nil)).Return(int64(0), nil)
	store.EXPECT().CountLibraryMoviesByState(ctx).Return([]sqlcdb.CountLibraryMoviesByStateRow{}, nil)

	response, err := svc.ListMoviesInLibrary(ctx, pagination.Params{Page: 1, PageSize: 50}, nil)
	require.NoError(t, err)
	assert.Equal(t, LibraryMoviesResponse{Items: []LibraryMovie{}, Pagination: pagination.Meta{Page: 1, PageSize: 50}, ByState: map[string]int64{}}, response)
}

func TestMovieService_ListMoviesInLibrary_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	svc, store, _, _ := newTestMovieService(ctrl)

	store.EXPECT().ListLibraryMovies(ctx, 0, 50, []storage.MovieState(nil)).Return(nil, errors.New("db error"))

	response, err := svc.ListMoviesInLibrary(ctx, pagination.Params{Page: 1, PageSize: 50}, nil)
	assert.Error(t, err)
	assert.Equal(t, LibraryMoviesResponse{}, response)
}

// ---------------------------------------------------------------------------
// Search & Detail Tests
// ---------------------------------------------------------------------------

func TestMovieService_SearchMovie_EmptyQuery(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	svc, _, _, _ := newTestMovieService(ctrl)

	_, err := svc.SearchMovie(ctx, "", 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "query is empty")
}

func TestMovieService_SearchMovie_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	svc, _, tmdbClient, _ := newTestMovieService(ctrl)

	searchResult := map[string]interface{}{"results": []interface{}{}}
	body, _ := json.Marshal(searchResult)
	httpResponse := &http.Response{
		StatusCode: 200,
		Status:     "200 OK",
		Body:       io.NopCloser(bytes.NewReader(body)),
	}

	tmdbClient.EXPECT().SearchMovie(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, params *tmdb.SearchMovieParams, _ ...tmdb.RequestEditorFn) (*http.Response, error) {
		assert.Equal(t, "test movie", params.Query)
		require.NotNil(t, params.Page)
		assert.Equal(t, int32(2), *params.Page)
		return httpResponse, nil
	})

	result, err := svc.SearchMovie(ctx, "test movie", 2)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestMovieService_SearchMovie_TMDBError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	svc, _, tmdbClient, _ := newTestMovieService(ctrl)

	tmdbClient.EXPECT().SearchMovie(ctx, gomock.Any()).Return(nil, errors.New("tmdb error"))

	result, err := svc.SearchMovie(ctx, "test movie", 1)
	require.Error(t, err)
	assert.Nil(t, result)
}

func TestMovieService_GetMovieDetailByTMDBID_NotInLibrary(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	svc, store, _, _ := newTestMovieService(ctrl)

	releaseDate := parseTestDate("2023-01-15")
	year := int32(2023)
	metadata := &sqlcdb.MovieMetadatum{
		ID:            1,
		TmdbID:        123,
		Title:         "Test Movie",
		OriginalTitle: sql.NullString{String: "Original Test Movie", Valid: true},
		Overview:      sql.NullString{String: "Test movie overview", Valid: true},
		Images:        "poster.jpg",
		Runtime:       120,
		Genres:        sql.NullString{String: "Action, Drama", Valid: true},
		Studio:        sql.NullString{String: "Test Studio", Valid: true},
		Website:       sql.NullString{String: "https://test.com", Valid: true},
		Popularity:    sql.NullFloat64{Float64: 8.5, Valid: true},
		Year:          sql.NullInt64{Int64: int64(year), Valid: true},
		ReleaseDate:   sql.NullTime{Time: releaseDate, Valid: true},
	}

	svc.metadataProvider = &mockMovieMetadataProvider{metadata: metadata}

	store.EXPECT().GetMovieByMetadataID(ctx, int64(1)).Return(sqlcdb.GetMovieByMetadataIDRow{}, storage.ErrNotFound)

	result, err := svc.GetMovieDetailByTMDBID(ctx, 123)
	require.NoError(t, err)
	require.NotNil(t, result)

	expected := &MovieDetailResult{
		TMDBID:        123,
		OriginalTitle: ptr.To("Original Test Movie"),
		Title:         "Test Movie",
		Overview:      ptr.To("Test movie overview"),
		PosterPath:    "poster.jpg",
		Runtime:       ptr.To(int32(120)),
		Genres:        ptr.To("Action, Drama"),
		Studio:        ptr.To("Test Studio"),
		Website:       ptr.To("https://test.com"),
		Popularity:    ptr.To(8.5),
		Year:          &year,
		ReleaseDate:   ptr.To("2023-01-15"),
		LibraryStatus: "Not In Library",
	}
	assert.Equal(t, expected, result)
}

func TestMovieService_GetMovieDetailByTMDBID_InLibrary(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	svc, store, _, _ := newTestMovieService(ctrl)

	metadata := &sqlcdb.MovieMetadatum{
		ID:     1,
		TmdbID: 123,
		Title:  "Test Movie",
		Images: "poster.jpg",
	}

	svc.metadataProvider = &mockMovieMetadataProvider{metadata: metadata}

	path := "/movies/test-movie"
	qualityProfileID := int64(1 << 41)
	movie := sqlcdb.GetMovieByMetadataIDRow{
		Movie: sqlcdb.Movie{
			ID:               1 << 40,
			MovieMetadataID:  sql.NullInt64{Int64: 1, Valid: true},
			Path:             sql.NullString{String: path, Valid: true},
			QualityProfileID: qualityProfileID,
			Monitored:        1,
		},
		State: string(storage.MovieStateDownloaded),
	}

	store.EXPECT().GetMovieByMetadataID(ctx, int64(1)).Return(movie, nil)

	result, err := svc.GetMovieDetailByTMDBID(ctx, 123)
	require.NoError(t, err)
	require.NotNil(t, result)

	monitored := true
	expected := &MovieDetailResult{
		TMDBID:           123,
		Title:            "Test Movie",
		PosterPath:       "poster.jpg",
		Runtime:          ptr.To(int32(0)),
		LibraryStatus:    string(storage.MovieStateDownloaded),
		ID:               ptr.To(int64(1 << 40)),
		Path:             &path,
		QualityProfileID: &qualityProfileID,
		Monitored:        &monitored,
	}
	assert.Equal(t, expected, result)
}

func TestMovieService_GetMovieDetailByTMDBID_MetadataError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	svc, _, _, _ := newTestMovieService(ctrl)

	svc.metadataProvider = &mockMovieMetadataProvider{err: errors.New("metadata error")}

	result, err := svc.GetMovieDetailByTMDBID(ctx, 123)
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestMovieService_GetMovieDetailByTMDBID_NilReleaseDate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	svc, store, _, _ := newTestMovieService(ctrl)

	metadata := &sqlcdb.MovieMetadatum{
		ID:          1,
		TmdbID:      123,
		Title:       "Test Movie",
		Images:      "poster.jpg",
		Runtime:     120,
		ReleaseDate: sql.NullTime{},
	}

	svc.metadataProvider = &mockMovieMetadataProvider{metadata: metadata}

	store.EXPECT().GetMovieByMetadataID(ctx, int64(1)).Return(sqlcdb.GetMovieByMetadataIDRow{}, storage.ErrNotFound)

	result, err := svc.GetMovieDetailByTMDBID(ctx, 123)
	require.NoError(t, err)
	require.NotNil(t, result)

	expected := &MovieDetailResult{
		TMDBID:        123,
		Title:         "Test Movie",
		PosterPath:    "poster.jpg",
		Runtime:       ptr.To(int32(120)),
		LibraryStatus: "Not In Library",
	}
	assert.Equal(t, expected, result)
}

// parseTestDate is a test helper to parse a date string.
func parseTestDate(s string) time.Time {
	t, _ := time.Parse("2006-01-02", s)
	return t
}
