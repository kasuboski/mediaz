package manager

import (
	"context"
	"testing"

	"github.com/kasuboski/mediaz/config"
	"github.com/kasuboski/mediaz/pkg/storage"
	storeMocks "github.com/kasuboski/mediaz/pkg/storage/mocks"
	"github.com/kasuboski/mediaz/pkg/tmdb"
	tmdbMock "github.com/kasuboski/mediaz/pkg/tmdb/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestMediaManager_GetSeriesMetadata(t *testing.T) {
	t.Run("get series metadata", func(t *testing.T) {
		ctx := context.Background()
		ctrl := gomock.NewController(t)

		store := storeMocks.NewMockStorage(ctrl)
		store.EXPECT().GetSeriesMetadata(ctx, gomock.Any()).Return(nil, storage.ErrNotFound)

		tmdbHttpMock := tmdbMock.NewMockITmdb(ctrl)
		tmdbHttpMock.EXPECT().GetSeriesDetails(gomock.Any(), gomock.Any()).Return(&tmdb.SeriesDetails{
			ID:           1,
			Name:         "Test Series",
			FirstAirDate: "2023-01-01",
		}, nil)

		m := MediaManager{
			tmdb:    tmdbHttpMock,
			library: nil,
			storage: store,
			factory: nil,
			configs: config.Manager{},
		}

		details, err := m.GetSeriesMetadata(ctx, 0)
		require.NoError(t, err)

		require.NotNil(t, details)
		require.Equal(t, "Test Series", details.Title)
		require.Equal(t, int32(1), details.TmdbID)
	})
}
