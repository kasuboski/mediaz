package manager

import (
	"context"
	"database/sql"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kasuboski/mediaz/config"
	"github.com/kasuboski/mediaz/internal/testsupport/faketmdb"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite"
	"github.com/kasuboski/mediaz/pkg/tmdb"
	"github.com/stretchr/testify/require"
)

func fakeTMDBMovieManager(t *testing.T) (MediaManager, *faketmdb.Server, context.Context) {
	t.Helper()
	ctx := context.Background()
	fake := faketmdb.New()
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)
	client, err := tmdb.New(server.URL, "integration-test-token")
	require.NoError(t, err)
	store, err := sqlite.New(ctx, ":memory:")
	require.NoError(t, err)
	return New(client, nil, nil, store, nil, config.Manager{}, config.Config{}), fake, ctx
}

func fakeTMDBMovieQualityProfile(t *testing.T, m MediaManager, ctx context.Context) int64 {
	t.Helper()
	profile, err := m.AddQualityProfile(ctx, AddQualityProfileRequest{Name: "integration", QualityIDs: []int64{3, 7}})
	require.NoError(t, err)
	return profile.ID
}

func TestFakeTMDBMovieSearchPaginationFilterAndNoMatch(t *testing.T) {
	m, fake, ctx := fakeTMDBMovieManager(t)
	first, err := m.SearchMovie(ctx, "a", 1)
	require.NoError(t, err)
	require.NotNil(t, first.Page)
	require.Equal(t, 1, *first.Page)
	require.Len(t, first.Results, 14)
	require.Equal(t, 14, *first.TotalResults)
	require.Equal(t, 1, *first.TotalPages)
	require.Equal(t, int64(1001), int64(*first.Results[0].ID))
	second, err := m.SearchMovie(ctx, "a", 2)
	require.NoError(t, err)
	require.NotNil(t, second.Page)
	require.Equal(t, 2, *second.Page)
	require.Empty(t, second.Results)
	noMatch, err := m.SearchMovie(ctx, "no such synthetic title", 1)
	require.NoError(t, err)
	require.Empty(t, noMatch.Results)

	requests := fake.Requests()
	require.Len(t, requests, 3)
	require.Equal(t, "/3/search/movie", requests[0].Path)
	require.Contains(t, requests[0].Query, "query=a")
	require.Contains(t, requests[0].Query, "page=1")
	require.Contains(t, requests[1].Query, "page=2")
	require.Contains(t, requests[2].Query, "query=no+such+synthetic+title")
}

func TestFakeTMDBMovieAddDetailAndPersistedMetadata(t *testing.T) {
	m, fake, ctx := fakeTMDBMovieManager(t)
	profileID := fakeTMDBMovieQualityProfile(t, m, ctx)
	added, err := m.AddMovieToLibrary(ctx, AddMovieRequest{TMDBID: 1001, QualityProfileID: profileID})
	require.NoError(t, err)
	detail, err := m.GetMovieDetailByTMDBID(ctx, 1001)
	require.NoError(t, err)
	require.Equal(t, int64(1001), detail.TMDBID)
	require.Equal(t, "The Glass Harbor", detail.Title)
	exact, err := m.SearchMovie(ctx, "Glass Harbor", 1)
	require.NoError(t, err)
	require.Len(t, exact.Results, 1)
	require.Equal(t, "/aOyE420zuFq9zWtEWjIccAiTrzU.jpg", detail.PosterPath)
	require.Equal(t, "2004-06-14", *detail.ReleaseDate)

	storedMovie, err := m.movieStorage.GetMovie(ctx, added.Movie.ID)
	require.NoError(t, err)
	require.Equal(t, *added, storedMovie)
	metadata, err := m.GetMovieMetadata(ctx, 1001)
	require.NoError(t, err)
	require.NotNil(t, metadata)
	require.Equal(t, sqlcdb.MovieMetadatum{ID: metadata.ID, TmdbID: 1001, ImdbID: sql.NullString{String: "tt90001001", Valid: true}, Images: "/aOyE420zuFq9zWtEWjIccAiTrzU.jpg", Genres: sql.NullString{String: "Drama", Valid: true}, Title: "The Glass Harbor", OriginalTitle: sql.NullString{String: "The Glass Harbor", Valid: true}, Runtime: 92, ReleaseDate: sql.NullTime{Time: time.Date(2004, time.June, 14, 0, 0, 0, 0, time.UTC), Valid: true}, Year: sql.NullInt64{Int64: 2004, Valid: true}, Overview: sql.NullString{String: "A synthetic story about The Glass Harbor.", Valid: true}, Popularity: sql.NullFloat64{Float64: 12.399999618530273, Valid: true}}, *metadata)

	requests := fake.Requests()
	require.Len(t, requests, 2)
	require.Equal(t, "GET", requests[0].Method)
	require.Equal(t, "/3/movie/1001", requests[0].Path)
}

func TestFakeTMDBMovieUnknownIDAndServerError(t *testing.T) {
	t.Run("unknown ID does not persist invalid metadata", func(t *testing.T) {
		m, fake, ctx := fakeTMDBMovieManager(t)
		_, err := m.AddMovieToLibrary(ctx, AddMovieRequest{TMDBID: 999999, QualityProfileID: fakeTMDBMovieQualityProfile(t, m, ctx)})
		require.Error(t, err)
		_, err = m.movieMetaStorage.GetMovieMetadataByTmdbID(ctx, 999999)
		require.ErrorIs(t, err, storage.ErrNotFound)
		_, err = m.movieStorage.GetMovieByMetadataID(ctx, 999999)
		require.Error(t, err)
		require.Equal(t, "/3/movie/999999", fake.Requests()[0].Path)
	})

	t.Run("server error propagates and does not persist invalid metadata", func(t *testing.T) {
		m, fake, ctx := fakeTMDBMovieManager(t)
		fake.SetStatus("/3/movie/1001", 500)
		_, err := m.AddMovieToLibrary(ctx, AddMovieRequest{TMDBID: 1001, QualityProfileID: fakeTMDBMovieQualityProfile(t, m, ctx)})
		require.Error(t, err)
		_, err = m.movieMetaStorage.GetMovieMetadataByTmdbID(ctx, 1001)
		require.ErrorIs(t, err, storage.ErrNotFound)
		require.Equal(t, "/3/movie/1001", fake.Requests()[0].Path)
	})
}
