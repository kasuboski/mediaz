package manager

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"testing/fstest"
	"time"

	downloadMock "github.com/kasuboski/mediaz/pkg/download/mocks"
	"github.com/kasuboski/mediaz/pkg/library"
	"github.com/kasuboski/mediaz/pkg/prowlarr"
	prowlMock "github.com/kasuboski/mediaz/pkg/prowlarr/mocks"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/kasuboski/mediaz/pkg/tmdb"
	"github.com/kasuboski/mediaz/pkg/tmdb/mocks"
	"github.com/oapi-codegen/nullable"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestAddMovietoLibrary(t *testing.T) {
	ctrl := gomock.NewController(t)
	tmdbMock := mocks.NewMockClientInterface(ctrl)

	store, err := sqlite.New(":memory:")
	require.NoError(t, err)

	schemas, err := storage.ReadSchemaFiles("../storage/sqlite/schema/schema.sql", "../storage/sqlite/schema/defaults.sql")
	require.NoError(t, err)

	ctx := context.Background()
	err = store.Init(ctx, schemas...)
	require.NoError(t, err)

	// create a date in the past
	releaseDate := time.Now().AddDate(0, 0, -1).Format(tmdb.ReleaseDateFormat)
	tmdbMock.EXPECT().MovieDetails(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(mediaDetailsResponse("test movie", 120, releaseDate), nil).Times(1)

	prowlarrMock := prowlMock.NewMockClientInterface(ctrl)
	// indexers := []Indexer{{ID: 1, Name: "test", Priority: 1}, {ID: 3, Name: "test2", Priority: 10}}
	// prowlarrMock.EXPECT().GetAPIV1Indexer(gomock.Any()).Return(indexersResponse(t, indexers), nil).Times(1)

	// bigSeeders := nullable.NewNullNullable[int32]()
	// bigSeeders.Set(23)

	// smallerSeeders := nullable.NewNullNullable[int32]()
	// smallerSeeders.Set(15)

	// smallestSeeders := nullable.NewNullNullable[int32]()
	// smallestSeeders.Set(10)

	// torrentProto := protocolPtr(prowlarr.DownloadProtocolTorrent)
	// usenetProto := protocolPtr(prowlarr.DownloadProtocolUsenet)

	// wantRelease := &prowlarr.ReleaseResource{ID: intPtr(123), Title: nullable.NewNullableWithValue("test movie"), Size: sizeGBToBytes(23), Seeders: bigSeeders, Protocol: torrentProto}
	// doNotWantRelease := &prowlarr.ReleaseResource{ID: intPtr(124), Title: nullable.NewNullableWithValue("test movie"), Size: sizeGBToBytes(23), Seeders: smallerSeeders, Protocol: torrentProto}
	// smallMovie := &prowlarr.ReleaseResource{ID: intPtr(125), Title: nullable.NewNullableWithValue("test movie - very small"), Size: sizeGBToBytes(1), Seeders: smallestSeeders, Protocol: torrentProto}
	// nzbMovie := &prowlarr.ReleaseResource{ID: intPtr(1225), Title: nullable.NewNullableWithValue("test movie - nzb"), Size: sizeGBToBytes(23), Seeders: smallestSeeders, Protocol: usenetProto}

	// releases := []*prowlarr.ReleaseResource{doNotWantRelease, wantRelease, smallMovie, nzbMovie}
	// prowlarrMock.EXPECT().GetAPIV1Search(gomock.Any(), gomock.Any()).Return(searchIndexersResponse(t, releases), nil).Times(len(indexers))

	downloadClient := model.DownloadClient{
		Implementation: "transmission",
		Type:           "torrent",
		Port:           8080,
		Host:           "transmission",
		Scheme:         "http",
	}

	downloadClientID, err := store.CreateDownloadClient(ctx, downloadClient)
	require.NoError(t, err)

	downloadClient.ID = int32(downloadClientID)

	movieFS := fstest.MapFS{}
	tvFS := fstest.MapFS{}
	lib := library.New(movieFS, tvFS)
	pClient, err := prowlarr.New(":", "1234")
	pClient.ClientInterface = prowlarrMock
	require.NoError(t, err)

	tClient, err := tmdb.New(":", "1234")
	tClient.ClientInterface = tmdbMock
	require.NoError(t, err)

	mockFactory := downloadMock.NewMockFactory(ctrl)
	// mockDownloadClient := downloadMock.NewMockDownloadClient(ctrl)
	// mockDownloadClient.EXPECT().Add(ctx, download.AddRequest{Release: wantRelease}).Times(1).Return(download.Status{
	// 	ID:   "123",
	// 	Name: "test download",
	// }, nil)

	// mockFactory.EXPECT().NewDownloadClient(downloadClient).Times(1).Return(mockDownloadClient, nil)

	m := New(tClient, pClient, lib, store, mockFactory)
	require.NotNil(t, m)

	req := AddMovieRequest{
		TMDBID:           1234,
		QualityProfileID: 1,
	}

	mov, err := m.AddMovieToLibrary(ctx, req)
	require.NoError(t, err)
	assert.NotNil(t, mov)

	assert.Equal(t, int32(1), mov.ID)
	assert.Equal(t, storage.MovieStateMissing, mov.State)

	// err = m.ReconcileMovies(ctx)
	// require.NoError(t, err)

	movie, err := m.storage.GetMovie(ctx, int64(mov.ID))
	require.Nil(t, err)

	movieMetadataID := int32(1)

	assert.Equal(t, &storage.Movie{
		Movie: model.Movie{
			ID:               1,
			Monitored:        1,
			QualityProfileID: 1,
			MovieMetadataID:  &movieMetadataID,
		},
		State: storage.MovieStateMissing,
	}, movie)
}

func TestIndexMovieLibrary(t *testing.T) {
	ctrl := gomock.NewController(t)
	tmdbMock := mocks.NewMockClientInterface(ctrl)
	prowlarrMock := prowlMock.NewMockClientInterface(ctrl)
	store, err := sqlite.New(":memory:")
	require.Nil(t, err)

	schemas, err := storage.ReadSchemaFiles("../storage/sqlite/schema/schema.sql")
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
	require.NoError(t, err)

	tClient, err := tmdb.New(":", "1234")
	tClient.ClientInterface = tmdbMock
	require.NoError(t, err)

	mockFactory := downloadMock.NewMockFactory(ctrl)
	m := New(tClient, pClient, lib, store, mockFactory)
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

func TestRun(t *testing.T) {
	ctrl := gomock.NewController(t)
	tmdbMock := mocks.NewMockClientInterface(ctrl)
	prowlarrMock := prowlMock.NewMockClientInterface(ctrl)
	store, err := sqlite.New(":memory:")
	require.Nil(t, err)

	schemas, err := storage.ReadSchemaFiles("../storage/sqlite/schema/schema.sql")
	require.Nil(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	defer cancel()
	err = store.Init(ctx, schemas...)
	require.Nil(t, err)

	movieFS, expectedMovies := library.MovieFSFromFile(t, "../library/test_movies.txt")
	require.NotEmpty(t, expectedMovies)
	tvFS, expectedEpisodes := library.TVFSFromFile(t, "../library/test_episodes.txt")
	require.NotEmpty(t, expectedEpisodes)

	lib := library.New(movieFS, tvFS)
	pClient, err := prowlarr.New(":", "1234")
	pClient.ClientInterface = prowlarrMock
	require.NoError(t, err)

	tClient, err := tmdb.New(":", "1234")
	tClient.ClientInterface = tmdbMock
	require.NoError(t, err)

	mockFactory := downloadMock.NewMockFactory(ctrl)
	m := New(tClient, pClient, lib, store, mockFactory)
	require.NotNil(t, m)

	err = m.Run(ctx)
	assert.ErrorIs(t, err, context.DeadlineExceeded)

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
func mediaDetailsResponse(title string, runtime int, releaseDate string) *http.Response {
	details := &tmdb.MediaDetails{
		ID:          1,
		Title:       &title,
		Runtime:     &runtime,
		ReleaseDate: &releaseDate,
	}

	b, _ := json.Marshal(details)

	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBuffer(b)),
	}
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

func protocolPtr(proto prowlarr.DownloadProtocol) *prowlarr.DownloadProtocol {
	return &proto
}
