package manager

import (
	"testing"
	"testing/fstest"

	"github.com/kasuboski/mediaz/pkg/library"
	prowlMock "github.com/kasuboski/mediaz/pkg/prowlarr/mocks"
	storeMock "github.com/kasuboski/mediaz/pkg/storage/mocks"
	"github.com/kasuboski/mediaz/pkg/tmdb/mocks"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestAddMovietoLibrary(t *testing.T) {
	ctrl := gomock.NewController(t)
	tmdbMock := mocks.NewMockClientInterface(ctrl)
	// tmdbMock.EXPECT().MovieDetails(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&http.Response{}, nil).AnyTimes()

	prowlarrMock := prowlMock.NewMockClientInterface(ctrl)
	storageMock := storeMock.NewMockStorage(ctrl)
	movieFS := fstest.MapFS{}
	tvFS := fstest.MapFS{}
	lib := library.New(movieFS, tvFS)
	m := New(tmdbMock, prowlarrMock, lib, storageMock)
	assert.NotNil(t, m)
	// ctx := context.Background()
	// req := AddMovieRequest{
	// 	TMDBID: 1234,
	// }
	// release, err := m.AddMovieToLibrary(ctx, req)
	// assert.Nil(t, err)

	// assert.NotNil(t, release)
}
