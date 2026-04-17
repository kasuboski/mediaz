package manager

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

	mockLibrary "github.com/kasuboski/mediaz/pkg/library/mocks"
	"github.com/kasuboski/mediaz/pkg/ptr"
	"github.com/kasuboski/mediaz/pkg/storage"
	storageMocks "github.com/kasuboski/mediaz/pkg/storage/mocks"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	tmdbMocks "github.com/kasuboski/mediaz/pkg/tmdb/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// mockMovieMetadataProvider is a test double for MovieMetadataProvider
type mockMovieMetadataProvider struct {
	metadata    *model.MovieMetadata
	metadataMap map[int32]*model.MovieMetadata
	err         error
}

func (m *mockMovieMetadataProvider) GetMovieMetadata(ctx context.Context, tmdbID int) (*model.MovieMetadata, error) {
	return m.metadata, m.err
}

func (m *mockMovieMetadataProvider) GetMovieMetadataByID(ctx context.Context, metadataID int32) (*model.MovieMetadata, error) {
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

	svc := NewMovieService(tmdbClient, lib, store, store, qualityService, metadataProvider)
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

	svc := NewMovieService(tmdbClient, lib, store, store, qualityService, metadataProvider)
	require.NotNil(t, svc)
	assert.Equal(t, tmdbClient, svc.tmdb)
	assert.Equal(t, lib, svc.library)
	assert.NotNil(t, svc.qualityService)
	assert.Equal(t, metadataProvider, svc.metadataProvider)
}

// ---------------------------------------------------------------------------
// CRUD Tests
// ---------------------------------------------------------------------------

func TestMovieService_UpdateMovieMonitored(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	svc, store, _, _ := newTestMovieService(ctrl)

	movie := &storage.Movie{
		Movie: model.Movie{
			ID:        1,
			Monitored: 1,
		},
	}

	store.EXPECT().UpdateMovie(ctx, gomock.Any(), gomock.Any()).Return(nil)
	store.EXPECT().GetMovie(ctx, int64(1)).Return(movie, nil)

	result, err := svc.UpdateMovieMonitored(ctx, 1, true)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, int32(1), result.ID)
}

func TestMovieService_UpdateMovieMonitored_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	svc, store, _, _ := newTestMovieService(ctrl)

	store.EXPECT().UpdateMovie(ctx, gomock.Any(), gomock.Any()).Return(errors.New("db error"))

	result, err := svc.UpdateMovieMonitored(ctx, 1, true)
	require.Error(t, err)
	assert.Nil(t, result)
}

func TestMovieService_UpdateMovieQualityProfile(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	svc, store, _, _ := newTestMovieService(ctrl)

	movie := &storage.Movie{
		Movie: model.Movie{
			ID:               1,
			QualityProfileID: 3,
		},
	}

	store.EXPECT().UpdateMovieQualityProfile(ctx, int64(1), int32(3)).Return(nil)
	store.EXPECT().GetMovie(ctx, int64(1)).Return(movie, nil)

	result, err := svc.UpdateMovieQualityProfile(ctx, 1, 3)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, int32(3), result.QualityProfileID)
}

func TestMovieService_UpdateMovieQualityProfile_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	svc, store, _, _ := newTestMovieService(ctrl)

	store.EXPECT().UpdateMovieQualityProfile(ctx, int64(1), int32(3)).Return(errors.New("db error"))

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
	movie := &storage.Movie{
		Movie: model.Movie{
			ID:   1,
			Path: &path,
		},
	}

	store.EXPECT().GetMovie(ctx, int64(1)).Return(movie, nil)
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
	movie := &storage.Movie{
		Movie: model.Movie{
			ID:   1,
			Path: &path,
		},
	}

	store.EXPECT().GetMovie(ctx, int64(1)).Return(movie, nil)
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

	store.EXPECT().GetMovie(ctx, int64(1)).Return(nil, storage.ErrNotFound)

	err := svc.DeleteMovie(ctx, 1, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get movie")
}

func TestMovieService_DeleteMovie_NilPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	svc, store, _, _ := newTestMovieService(ctrl)

	movie := &storage.Movie{
		Movie: model.Movie{
			ID:   1,
			Path: nil,
		},
	}

	store.EXPECT().GetMovie(ctx, int64(1)).Return(movie, nil)

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

	metadataID := int32(1)
	path := "movie1"
	discoveredMovie := &storage.Movie{
		Movie: model.Movie{
			MovieMetadataID: &metadataID,
			Path:            &path,
		},
		State: storage.MovieStateDiscovered,
	}

	downloadedMovie := &storage.Movie{
		Movie: model.Movie{
			MovieMetadataID: &metadataID,
			Path:            &path,
		},
		State: storage.MovieStateDownloaded,
	}

	year := int32(2024)
	movieMetadata := &model.MovieMetadata{
		ID:     1,
		TmdbID: 123,
		Title:  "Test Movie",
		Images: "poster.jpg",
		Year:   &year,
	}

	svc.metadataProvider = &mockMovieMetadataProvider{
		metadata:    movieMetadata,
		metadataMap: map[int32]*model.MovieMetadata{1: movieMetadata},
	}

	store.EXPECT().ListMovies(ctx).Return([]*storage.Movie{discoveredMovie, downloadedMovie}, nil)

	movies, err := svc.ListMoviesInLibrary(ctx)
	require.NoError(t, err)
	assert.Len(t, movies, 2)

	expectedMovie := LibraryMovie{
		Path:       path,
		TMDBID:     movieMetadata.TmdbID,
		Title:      movieMetadata.Title,
		PosterPath: movieMetadata.Images,
		Year:       *movieMetadata.Year,
	}

	for _, movie := range movies {
		assert.Equal(t, expectedMovie.Path, movie.Path)
		assert.Equal(t, expectedMovie.TMDBID, movie.TMDBID)
		assert.Equal(t, expectedMovie.Title, movie.Title)
		assert.Equal(t, expectedMovie.PosterPath, movie.PosterPath)
		assert.Equal(t, expectedMovie.Year, movie.Year)
	}
}

func TestMovieService_ListMoviesInLibrary_Empty(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	svc, store, _, _ := newTestMovieService(ctrl)

	store.EXPECT().ListMovies(ctx).Return([]*storage.Movie{}, nil)

	movies, err := svc.ListMoviesInLibrary(ctx)
	require.NoError(t, err)
	assert.Empty(t, movies)
}

func TestMovieService_ListMoviesInLibrary_SkipsNoMetadata(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	svc, store, _, _ := newTestMovieService(ctrl)

	path := "movie1"
	discoveredMovie := &storage.Movie{
		Movie: model.Movie{
			Path: &path,
		},
		State: storage.MovieStateDiscovered,
	}

	store.EXPECT().ListMovies(ctx).Return([]*storage.Movie{discoveredMovie}, nil)

	movies, err := svc.ListMoviesInLibrary(ctx)
	require.NoError(t, err)
	assert.Empty(t, movies, "movies without metadata should not be included in library")
}

func TestMovieService_ListMoviesInLibrary_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	svc, store, _, _ := newTestMovieService(ctrl)

	store.EXPECT().ListMovies(ctx).Return(nil, errors.New("db error"))

	movies, err := svc.ListMoviesInLibrary(ctx)
	assert.Error(t, err)
	assert.Nil(t, movies)
}

// ---------------------------------------------------------------------------
// Search & Detail Tests
// ---------------------------------------------------------------------------

func TestMovieService_SearchMovie_EmptyQuery(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	svc, _, _, _ := newTestMovieService(ctrl)

	_, err := svc.SearchMovie(ctx, "")
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

	tmdbClient.EXPECT().SearchMovie(ctx, gomock.Any()).Return(httpResponse, nil)

	result, err := svc.SearchMovie(ctx, "test movie")
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestMovieService_SearchMovie_TMDBError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	svc, _, tmdbClient, _ := newTestMovieService(ctrl)

	tmdbClient.EXPECT().SearchMovie(ctx, gomock.Any()).Return(nil, errors.New("tmdb error"))

	result, err := svc.SearchMovie(ctx, "test movie")
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
	metadata := &model.MovieMetadata{
		ID:            1,
		TmdbID:        123,
		Title:         "Test Movie",
		OriginalTitle: ptr.To("Original Test Movie"),
		Overview:      ptr.To("Test movie overview"),
		Images:        "poster.jpg",
		Runtime:       120,
		Genres:        ptr.To("Action, Drama"),
		Studio:        ptr.To("Test Studio"),
		Website:       ptr.To("https://test.com"),
		Popularity:    ptr.To(8.5),
		Year:          &year,
		ReleaseDate:   &releaseDate,
	}

	svc.metadataProvider = &mockMovieMetadataProvider{metadata: metadata}

	store.EXPECT().GetMovieByMetadataID(ctx, 1).Return(nil, storage.ErrNotFound)

	result, err := svc.GetMovieDetailByTMDBID(ctx, 123)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, int32(123), result.TMDBID)
	assert.Equal(t, ptr.To("Original Test Movie"), result.OriginalTitle)
	assert.Equal(t, "Test Movie", result.Title)
	assert.Equal(t, ptr.To("Test movie overview"), result.Overview)
	assert.Equal(t, "poster.jpg", result.PosterPath)
	assert.Equal(t, ptr.To(int32(120)), result.Runtime)
	assert.Equal(t, ptr.To("Action, Drama"), result.Genres)
	assert.Equal(t, ptr.To("Test Studio"), result.Studio)
	assert.Equal(t, ptr.To("https://test.com"), result.Website)
	assert.Equal(t, ptr.To(8.5), result.Popularity)
	assert.Equal(t, &year, result.Year)
	assert.Equal(t, ptr.To("2023-01-15"), result.ReleaseDate)
	assert.Equal(t, "Not In Library", result.LibraryStatus)
	assert.Nil(t, result.Path)
	assert.Nil(t, result.QualityProfileID)
	assert.Nil(t, result.Monitored)
}

func TestMovieService_GetMovieDetailByTMDBID_InLibrary(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	svc, store, _, _ := newTestMovieService(ctrl)

	metadata := &model.MovieMetadata{
		ID:     1,
		TmdbID: 123,
		Title:  "Test Movie",
		Images: "poster.jpg",
	}

	svc.metadataProvider = &mockMovieMetadataProvider{metadata: metadata}

	path := "/movies/test-movie"
	qualityProfileID := int32(2)
	movie := &storage.Movie{
		Movie: model.Movie{
			ID:               1,
			MovieMetadataID:  ptr.To(int32(1)),
			Path:             &path,
			QualityProfileID: qualityProfileID,
			Monitored:        1,
		},
		State: storage.MovieStateDownloaded,
	}

	store.EXPECT().GetMovieByMetadataID(ctx, 1).Return(movie, nil)

	result, err := svc.GetMovieDetailByTMDBID(ctx, 123)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, int32(123), result.TMDBID)
	assert.Equal(t, "Test Movie", result.Title)
	assert.Equal(t, string(storage.MovieStateDownloaded), result.LibraryStatus)
	assert.Equal(t, &path, result.Path)
	assert.Equal(t, &qualityProfileID, result.QualityProfileID)
	monitored := true
	assert.Equal(t, &monitored, result.Monitored)
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

	metadata := &model.MovieMetadata{
		ID:          1,
		TmdbID:      123,
		Title:       "Test Movie",
		Images:      "poster.jpg",
		Runtime:     120,
		ReleaseDate: nil,
	}

	svc.metadataProvider = &mockMovieMetadataProvider{metadata: metadata}

	store.EXPECT().GetMovieByMetadataID(ctx, 1).Return(nil, storage.ErrNotFound)

	result, err := svc.GetMovieDetailByTMDBID(ctx, 123)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, int32(123), result.TMDBID)
	assert.Nil(t, result.ReleaseDate)
	assert.Equal(t, "Not In Library", result.LibraryStatus)
}

// parseTestDate is a test helper to parse a date string.
func parseTestDate(s string) time.Time {
	t, _ := time.Parse("2006-01-02", s)
	return t
}
