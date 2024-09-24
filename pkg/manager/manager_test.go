package manager

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"testing"
	"testing/fstest"

	"github.com/kasuboski/mediaz/pkg/library"
	"github.com/kasuboski/mediaz/pkg/prowlarr"
	prowlMock "github.com/kasuboski/mediaz/pkg/prowlarr/mocks"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite"
	"github.com/kasuboski/mediaz/pkg/tmdb/mocks"
	"github.com/oapi-codegen/nullable"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestAddMovietoLibrary(t *testing.T) {
	ctrl := gomock.NewController(t)
	tmdbMock := mocks.NewMockClientInterface(ctrl)
	tmdbMock.EXPECT().MovieDetails(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(mediaDetailsResponse("test movie", 120), nil).Times(1)

	prowlarrMock := prowlMock.NewMockClientInterface(ctrl)
	indexers := []Indexer{{ID: 1, Name: "test", Priority: 1}, {ID: 3, Name: "test2", Priority: 10}}
	prowlarrMock.EXPECT().GetAPIV1Indexer(gomock.Any()).Return(indexersResponse(t, indexers), nil).Times(1)

	bigSeeders := nullable.NewNullNullable[int32]()
	bigSeeders.Set(23)

	smallerSeeders := nullable.NewNullNullable[int32]()
	smallerSeeders.Set(15)

	smallestSeeders := nullable.NewNullNullable[int32]()
	smallestSeeders.Set(10)

	wantRelease := &prowlarr.ReleaseResource{ID: intPtr(123), Title: nullable.NewNullableWithValue("test movie"), Size: sizeGBToBytes(23), Seeders: bigSeeders}
	doNotWantRelease := &prowlarr.ReleaseResource{ID: intPtr(124), Title: nullable.NewNullableWithValue("test movie"), Size: sizeGBToBytes(23), Seeders: smallerSeeders}
	smallMovie := &prowlarr.ReleaseResource{ID: intPtr(125), Title: nullable.NewNullableWithValue("test movie - very small"), Size: sizeGBToBytes(1), Seeders: smallestSeeders}

	releases := []*prowlarr.ReleaseResource{doNotWantRelease, wantRelease, smallMovie}
	prowlarrMock.EXPECT().GetAPIV1Search(gomock.Any(), gomock.Any()).Return(searchIndexersResponse(t, releases), nil).Times(len(indexers))

	store, err := sqlite.New(":memory:")
	require.Nil(t, err)

	schemas, err := storage.ReadSchemaFiles("../storage/sqlite/schema/schema.sql", "../storage/sqlite/schema/defaults.sql")
	assert.Nil(t, err)

	ctx := context.Background()
	err = store.Init(ctx, schemas...)
	require.Nil(t, err)

	movieFS := fstest.MapFS{}
	tvFS := fstest.MapFS{}
	lib := library.New(movieFS, tvFS)
	pClient, err := prowlarr.New(":", "1234")
	pClient.ClientInterface = prowlarrMock
	require.Nil(t, err)
	m := New(tmdbMock, pClient, lib, store)
	require.NotNil(t, m)

	req := AddMovieRequest{
		TMDBID:           1234,
		QualityProfileID: 1,
	}
	release, err := m.AddMovieToLibrary(ctx, req)
	assert.Nil(t, err)

	assert.NotNil(t, release)
	assert.Equal(t, int32(123), *release.ID)
}

func TestIndexMovieLibrary(t *testing.T) {
	ctrl := gomock.NewController(t)
	tmdbMock := mocks.NewMockClientInterface(ctrl)
	prowlarrMock := prowlMock.NewMockClientInterface(ctrl)
	store, err := sqlite.New(":memory:")
	require.Nil(t, err)

	schemas, err := storage.ReadSchemaFiles("../../schema.sql")
	require.Nil(t, err)

	ctx := context.Background()
	err = store.Init(ctx, schemas...)
	require.Nil(t, err)

	movieFS, expectedMovies := library.MovieFSFromFile(t, "../library/test_movies.txt")
	require.NotEmpty(t, expectedMovies)
	tvFS, expectedEpisodes := library.TVFSFromFile(t, "../library/test_episodes.txt")
	require.NotEmpty(t, expectedEpisodes)

	lib := library.New(movieFS, tvFS)
	pClient, err := prowlarr.New(":", "1234")
	pClient.ClientInterface = prowlarrMock
	assert.Nil(t, err)
	m := New(tmdbMock, pClient, lib, store)
	require.NotNil(t, m)

	err = m.IndexMovieLibrary(ctx)
	require.Nil(t, err)

	mfs, err := store.ListMovieFiles(ctx)
	assert.Nil(t, err)
	assert.Len(t, mfs, len(expectedMovies))

	ms, err := store.ListMovies(ctx)
	assert.Nil(t, err)
	assert.Len(t, ms, len(expectedMovies))
}

func searchIndexersResponse(t *testing.T, releases []*prowlarr.ReleaseResource) *http.Response {
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(map[string][]string),
	}

	out, err := json.Marshal(releases)
	assert.Nil(t, err)

	resp.Body = io.NopCloser(bytes.NewBuffer(out))

	return resp
}

// mediaDetailsResponse returns an http.Response that represents a MediaDetails with the given title and runtime
func mediaDetailsResponse(title string, runtime int) *http.Response {
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(map[string][]string),
	}

	resp.Body = io.NopCloser(bytes.NewBufferString(`{"title":"` + title + `","runtime":` + strconv.Itoa(runtime) + `}`))
	return resp

}

func indexersResponse(t *testing.T, indexers []Indexer) *http.Response {
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(map[string][]string),
	}

	prowlarrIndexers := make([]*prowlarr.IndexerResource, len(indexers))
	for i, indexer := range indexers {
		prowlarrIndexers[i] = &prowlarr.IndexerResource{
			ID:           &indexer.ID,
			Name:         nullable.NewNullableWithValue(indexer.Name),
			Priority:     &indexer.Priority,
			Capabilities: &prowlarr.IndexerCapabilityResource{},
		}
	}
	out, err := json.Marshal(prowlarrIndexers)
	assert.Nil(t, err)
	resp.Body = io.NopCloser(bytes.NewBuffer(out))
	return resp
}

func sizeGBToBytes(gb int) *int64 {
	b := int64(gb * 1024 * 1024 * 1024)
	return &b
}
