package manager

import (
	"context"
	"testing"

	"github.com/kasuboski/mediaz/config"
	"github.com/kasuboski/mediaz/pkg/download"
	downloadMocks "github.com/kasuboski/mediaz/pkg/download/mocks"
	"github.com/kasuboski/mediaz/pkg/prowlarr"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/mocks"
	mediaSqlite "github.com/kasuboski/mediaz/pkg/storage/sqlite"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func newDownloadServiceStore(t *testing.T) storage.Storage {
	t.Helper()
	ctx := context.Background()
	store, err := mediaSqlite.New(ctx, ":memory:")
	require.NoError(t, err)
	schemas, err := storage.ReadSchemaFiles("../storage/sqlite/schema/schema.sql", "../storage/sqlite/schema/defaults.sql")
	require.NoError(t, err)
	err = store.Init(ctx, schemas...)
	require.NoError(t, err)
	return store
}

func TestDownloadService_GetQualityProfile(t *testing.T) {
	t.Run("movie quality profile", func(t *testing.T) {
		ctx := context.Background()
		ds := NewDownloadService(nil, newDownloadServiceStore(t), nil)

		profile, err := ds.GetQualityProfile(ctx, 1)
		require.NoError(t, err)
		want := storage.QualityProfile{
			ID:              1,
			Name:            "Standard Definition",
			CutoffQualityID: nil,
			UpgradeAllowed:  false,
			Qualities: []storage.QualityDefinition{
				{ID: 1, Name: "HDTV-720p", MediaType: "movie", PreferredSize: 1999, MinSize: 17.1, MaxSize: 2000},
				{ID: 2, Name: "WEBDL-720p", MediaType: "movie", PreferredSize: 1999, MinSize: 12.5, MaxSize: 2000},
			},
		}
		assert.Equal(t, want, profile)
	})

	t.Run("episode quality profile", func(t *testing.T) {
		ctx := context.Background()
		ds := NewDownloadService(nil, newDownloadServiceStore(t), nil)

		profile, err := ds.GetQualityProfile(ctx, 5)
		require.NoError(t, err)
		want := storage.QualityProfile{
			ID:              5,
			Name:            "High Definition",
			CutoffQualityID: nil,
			UpgradeAllowed:  false,
			Qualities: []storage.QualityDefinition{
				{ID: 23, Name: "Remux-1080p", MediaType: "episode", PreferredSize: 995, MinSize: 69.1, MaxSize: 1000},
				{ID: 22, Name: "Bluray-1080p", MediaType: "episode", PreferredSize: 995, MinSize: 50.4, MaxSize: 1000},
				{ID: 18, Name: "Bluray-720p", MediaType: "episode", PreferredSize: 995, MinSize: 17.1, MaxSize: 1000},
				{ID: 19, Name: "HDTV-1080p", MediaType: "episode", PreferredSize: 995, MinSize: 15, MaxSize: 1000},
				{ID: 20, Name: "WEBDL-1080p", MediaType: "episode", PreferredSize: 995, MinSize: 15, MaxSize: 1000},
				{ID: 21, Name: "WEBRip-1080p", MediaType: "episode", PreferredSize: 995, MinSize: 15, MaxSize: 1000},
				{ID: 17, Name: "WEBRip-720p", MediaType: "episode", PreferredSize: 995, MinSize: 10, MaxSize: 1000},
			},
		}
		assert.Equal(t, want, profile)
	})
}

func TestDownloadService_ListMovieQualityProfiles(t *testing.T) {
	ctx := context.Background()
	ds := NewDownloadService(nil, newDownloadServiceStore(t), nil)

	profiles, err := ds.ListMovieQualityProfiles(ctx)
	require.NoError(t, err)
	require.Len(t, profiles, 3)

	want := []*storage.QualityProfile{
		{
			ID: 3, Name: "Ultra High Definition", CutoffQualityID: nil, UpgradeAllowed: false,
			Qualities: []storage.QualityDefinition{
				{ID: 9, Name: "Remux-1080p", MediaType: "movie", PreferredSize: 1999, MinSize: 102, MaxSize: 2000},
				{ID: 13, Name: "Bluray-2160p", MediaType: "movie", PreferredSize: 1999, MinSize: 102, MaxSize: 2000},
				{ID: 10, Name: "HDTV-2160p", MediaType: "movie", PreferredSize: 1999, MinSize: 85, MaxSize: 2000},
				{ID: 11, Name: "WEBDL-2160p", MediaType: "movie", PreferredSize: 1999, MinSize: 34.5, MaxSize: 2000},
				{ID: 12, Name: "WEBRip-2160p", MediaType: "movie", PreferredSize: 1999, MinSize: 34.5, MaxSize: 2000},
			},
		},
		{
			ID: 2, Name: "High Definition", CutoffQualityID: nil, UpgradeAllowed: false,
			Qualities: []storage.QualityDefinition{
				{ID: 8, Name: "Bluray-1080p", MediaType: "movie", PreferredSize: 1999, MinSize: 50.8, MaxSize: 2000},
				{ID: 5, Name: "HDTV-1080p", MediaType: "movie", PreferredSize: 1999, MinSize: 33.8, MaxSize: 2000},
				{ID: 4, Name: "Bluray-720p", MediaType: "movie", PreferredSize: 1999, MinSize: 25.7, MaxSize: 2000},
				{ID: 3, Name: "WEBRip-720p", MediaType: "movie", PreferredSize: 1999, MinSize: 12.5, MaxSize: 2000},
				{ID: 6, Name: "WEBDL-1080p", MediaType: "movie", PreferredSize: 1999, MinSize: 12.5, MaxSize: 2000},
				{ID: 7, Name: "WEBRip-1080p", MediaType: "movie", PreferredSize: 1999, MinSize: 12.5, MaxSize: 2000},
			},
		},
		{
			ID: 1, Name: "Standard Definition", CutoffQualityID: nil, UpgradeAllowed: false,
			Qualities: []storage.QualityDefinition{
				{ID: 1, Name: "HDTV-720p", MediaType: "movie", PreferredSize: 1999, MinSize: 17.1, MaxSize: 2000},
				{ID: 2, Name: "WEBDL-720p", MediaType: "movie", PreferredSize: 1999, MinSize: 12.5, MaxSize: 2000},
			},
		},
	}
	assert.Equal(t, want, profiles)
}

func TestDownloadService_ListEpisodeQualityProfiles(t *testing.T) {
	ctx := context.Background()
	ds := NewDownloadService(nil, newDownloadServiceStore(t), nil)

	profiles, err := ds.ListEpisodeQualityProfiles(ctx)
	require.NoError(t, err)
	require.Len(t, profiles, 3)

	want := []*storage.QualityProfile{
		{
			ID: 6, Name: "Ultra High Definition", CutoffQualityID: nil, UpgradeAllowed: false,
			Qualities: []storage.QualityDefinition{
				{ID: 27, Name: "Bluray-2160p", MediaType: "episode", PreferredSize: 995, MinSize: 94.6, MaxSize: 1000},
				{ID: 23, Name: "Remux-1080p", MediaType: "episode", PreferredSize: 995, MinSize: 69.1, MaxSize: 1000},
				{ID: 24, Name: "HDTV-2160p", MediaType: "episode", PreferredSize: 995, MinSize: 25, MaxSize: 1000},
				{ID: 25, Name: "WEBDL-2160p", MediaType: "episode", PreferredSize: 995, MinSize: 25, MaxSize: 1000},
				{ID: 26, Name: "WEBRip-2160p", MediaType: "episode", PreferredSize: 995, MinSize: 25, MaxSize: 1000},
			},
		},
		{
			ID: 5, Name: "High Definition", CutoffQualityID: nil, UpgradeAllowed: false,
			Qualities: []storage.QualityDefinition{
				{ID: 23, Name: "Remux-1080p", MediaType: "episode", PreferredSize: 995, MinSize: 69.1, MaxSize: 1000},
				{ID: 22, Name: "Bluray-1080p", MediaType: "episode", PreferredSize: 995, MinSize: 50.4, MaxSize: 1000},
				{ID: 18, Name: "Bluray-720p", MediaType: "episode", PreferredSize: 995, MinSize: 17.1, MaxSize: 1000},
				{ID: 19, Name: "HDTV-1080p", MediaType: "episode", PreferredSize: 995, MinSize: 15, MaxSize: 1000},
				{ID: 20, Name: "WEBDL-1080p", MediaType: "episode", PreferredSize: 995, MinSize: 15, MaxSize: 1000},
				{ID: 21, Name: "WEBRip-1080p", MediaType: "episode", PreferredSize: 995, MinSize: 15, MaxSize: 1000},
				{ID: 17, Name: "WEBRip-720p", MediaType: "episode", PreferredSize: 995, MinSize: 10, MaxSize: 1000},
			},
		},
		{
			ID: 4, Name: "Standard Definition", CutoffQualityID: nil, UpgradeAllowed: false,
			Qualities: []storage.QualityDefinition{
				{ID: 15, Name: "HDTV-720p", MediaType: "episode", PreferredSize: 995, MinSize: 10, MaxSize: 1000},
				{ID: 16, Name: "WEBDL-720p", MediaType: "episode", PreferredSize: 995, MinSize: 10, MaxSize: 1000},
			},
		},
	}
	assert.Equal(t, want, profiles)
}

func TestDownloadService_UpdateQualityProfile(t *testing.T) {
	t.Run("update profile with new quality associations", func(t *testing.T) {
		ctx := context.Background()
		ds := NewDownloadService(nil, newDownloadServiceStore(t), nil)

		profile, err := ds.GetQualityProfile(ctx, 1)
		require.NoError(t, err)
		assert.Greater(t, len(profile.Qualities), 0)

		updated, err := ds.UpdateQualityProfile(ctx, 1, UpdateQualityProfileRequest{
			Name:           "Updated Profile",
			UpgradeAllowed: false,
			QualityIDs:     []int32{3, 7},
		})
		require.NoError(t, err)
		want := storage.QualityProfile{
			ID:              1,
			Name:            "Updated Profile",
			CutoffQualityID: nil,
			UpgradeAllowed:  false,
			Qualities: []storage.QualityDefinition{
				{ID: 3, Name: "WEBRip-720p", MediaType: "movie", PreferredSize: 1999, MinSize: 12.5, MaxSize: 2000},
				{ID: 7, Name: "WEBRip-1080p", MediaType: "movie", PreferredSize: 1999, MinSize: 12.5, MaxSize: 2000},
			},
		}
		assert.Equal(t, want, updated)
	})

	t.Run("fails with empty quality IDs", func(t *testing.T) {
		ctx := context.Background()
		ds := NewDownloadService(nil, newDownloadServiceStore(t), nil)

		_, err := ds.UpdateQualityProfile(ctx, 1, UpdateQualityProfileRequest{
			Name:       "Updated Profile",
			QualityIDs: []int32{},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least one quality must be selected")
	})

	t.Run("fails for non-existent profile", func(t *testing.T) {
		ctx := context.Background()
		ds := NewDownloadService(nil, newDownloadServiceStore(t), nil)

		_, err := ds.UpdateQualityProfile(ctx, 999, UpdateQualityProfileRequest{
			Name:       "Updated Profile",
			QualityIDs: []int32{3},
		})
		require.Error(t, err)
	})
}

func TestDownloadService_AddQualityProfile(t *testing.T) {
	t.Run("fails when upgrade allowed but cutoff not provided", func(t *testing.T) {
		ctx := context.Background()
		ds := NewDownloadService(nil, newDownloadServiceStore(t), nil)

		_, err := ds.AddQualityProfile(ctx, AddQualityProfileRequest{
			Name:           "Test Profile",
			UpgradeAllowed: true,
			QualityIDs:     []int32{3, 7},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cutoff quality must be specified when upgrades are allowed")
	})

	t.Run("succeeds when upgrade not allowed and cutoff not provided", func(t *testing.T) {
		ctx := context.Background()
		ds := NewDownloadService(nil, newDownloadServiceStore(t), nil)

		profile, err := ds.AddQualityProfile(ctx, AddQualityProfileRequest{
			Name:       "Test Profile",
			QualityIDs: []int32{3, 7},
		})
		require.NoError(t, err)
		want := storage.QualityProfile{
			ID:              profile.ID,
			Name:            "Test Profile",
			CutoffQualityID: nil,
			UpgradeAllowed:  false,
			Qualities: []storage.QualityDefinition{
				{ID: 3, Name: "WEBRip-720p", MediaType: "movie", PreferredSize: 1999, MinSize: 12.5, MaxSize: 2000},
				{ID: 7, Name: "WEBRip-1080p", MediaType: "movie", PreferredSize: 1999, MinSize: 12.5, MaxSize: 2000},
			},
		}
		assert.Equal(t, want, profile)
	})

	t.Run("fails when cutoff not in quality list", func(t *testing.T) {
		ctx := context.Background()
		ds := NewDownloadService(nil, newDownloadServiceStore(t), nil)

		cutoffID := int32(10)
		_, err := ds.AddQualityProfile(ctx, AddQualityProfileRequest{
			Name:            "Test Profile",
			CutoffQualityID: &cutoffID,
			UpgradeAllowed:  true,
			QualityIDs:      []int32{3, 7},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cutoff quality must be one of the selected qualities")
	})
}

func TestAvailableProtocols(t *testing.T) {
	clients := []*model.DownloadClient{
		{Type: "usenet"},
		{Type: "torrent"},
		{Type: "usenet"},
		{Type: "usenet"},
		{Type: "torrent"},
	}

	actual := availableProtocols(clients)
	assert.NotEmpty(t, actual)
	assert.Len(t, actual, 2)

	actual = availableProtocols([]*model.DownloadClient{})
	assert.Empty(t, actual)
}

func TestClientForProtocol(t *testing.T) {
	clients := []*model.DownloadClient{
		{ID: 1, Type: "usenet"},
		{ID: 2, Type: "torrent"},
		{ID: 3, Type: "usenet"},
		{ID: 4, Type: "usenet"},
		{ID: 5, Type: "torrent"},
	}

	t.Run("find torrent", func(t *testing.T) {
		actual := clientForProtocol(clients, prowlarr.DownloadProtocolTorrent)
		assert.NotNil(t, actual)
		assert.Equal(t, int32(2), actual.ID)
	})
	t.Run("find usenet", func(t *testing.T) {
		actual := clientForProtocol(clients, prowlarr.DownloadProtocolUsenet)
		assert.NotNil(t, actual)
		assert.Equal(t, int32(1), actual.ID)
	})

	t.Run("not found", func(t *testing.T) {
		actual := clientForProtocol([]*model.DownloadClient{{ID: 1, Type: "usenet"}}, prowlarr.DownloadProtocolTorrent)
		assert.Nil(t, actual)
	})

	t.Run("empty", func(t *testing.T) {
		actual := clientForProtocol([]*model.DownloadClient{}, prowlarr.DownloadProtocolTorrent)
		assert.Nil(t, actual)
	})
}

func TestUpdateDownloadClient(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	store := mocks.NewMockStorage(ctrl)
	m := New(nil, nil, nil, store, nil, config.Manager{}, config.Config{})

	t.Run("update with new API key", func(t *testing.T) {
		newApiKey := "new-api-key"
		request := UpdateDownloadClientRequest{
			DownloadClient: model.DownloadClient{
				Type:           "usenet",
				Implementation: "sabnzbd",
				Scheme:         "https",
				Host:           "sabnzbd.example.com",
				Port:           443,
				APIKey:         &newApiKey,
			},
		}

		store.EXPECT().UpdateDownloadClient(ctx, int64(1), gomock.Any()).Return(nil)

		result, err := m.UpdateDownloadClient(ctx, 1, request)
		require.NoError(t, err)
		assert.Equal(t, int32(1), result.ID)
		assert.Equal(t, "usenet", result.Type)
		assert.Equal(t, &newApiKey, result.APIKey)
	})

	t.Run("update preserves existing API key when not provided", func(t *testing.T) {
		existingApiKey := "existing-key"
		existingClient := model.DownloadClient{
			ID:             1,
			Type:           "usenet",
			Implementation: "sabnzbd",
			Scheme:         "http",
			Host:           "localhost",
			Port:           8080,
			APIKey:         &existingApiKey,
		}

		emptyKey := ""
		request := UpdateDownloadClientRequest{
			DownloadClient: model.DownloadClient{
				Type:           "usenet",
				Implementation: "sabnzbd",
				Scheme:         "https",
				Host:           "sabnzbd.example.com",
				Port:           443,
				APIKey:         &emptyKey,
			},
		}

		store.EXPECT().GetDownloadClient(ctx, int64(1)).Return(existingClient, nil)
		store.EXPECT().UpdateDownloadClient(ctx, int64(1), gomock.Any()).DoAndReturn(
			func(ctx context.Context, id int64, client model.DownloadClient) error {
				assert.Equal(t, &existingApiKey, client.APIKey)
				return nil
			},
		)

		result, err := m.UpdateDownloadClient(ctx, 1, request)
		require.NoError(t, err)
		assert.Equal(t, &existingApiKey, result.APIKey)
	})

	t.Run("update preserves API key when nil", func(t *testing.T) {
		existingApiKey := "existing-key"
		existingClient := model.DownloadClient{
			ID:             1,
			Type:           "usenet",
			Implementation: "sabnzbd",
			Scheme:         "http",
			Host:           "localhost",
			Port:           8080,
			APIKey:         &existingApiKey,
		}

		request := UpdateDownloadClientRequest{
			DownloadClient: model.DownloadClient{
				Type:           "usenet",
				Implementation: "sabnzbd",
				Scheme:         "https",
				Host:           "sabnzbd.example.com",
				Port:           443,
				APIKey:         nil,
			},
		}

		store.EXPECT().GetDownloadClient(ctx, int64(1)).Return(existingClient, nil)
		store.EXPECT().UpdateDownloadClient(ctx, int64(1), gomock.Any()).DoAndReturn(
			func(ctx context.Context, id int64, client model.DownloadClient) error {
				assert.Equal(t, &existingApiKey, client.APIKey)
				return nil
			},
		)

		result, err := m.UpdateDownloadClient(ctx, 1, request)
		require.NoError(t, err)
		assert.Equal(t, &existingApiKey, result.APIKey)
	})
}

func TestTestDownloadClient(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	factory := downloadMocks.NewMockFactory(ctrl)
	m := New(nil, nil, nil, nil, factory, config.Manager{}, config.Config{})

	t.Run("successful connection test", func(t *testing.T) {
		client := downloadMocks.NewMockDownloadClient(ctrl)
		request := AddDownloadClientRequest{
			DownloadClient: model.DownloadClient{
				Type:           "torrent",
				Implementation: "transmission",
				Scheme:         "http",
				Host:           "localhost",
				Port:           9091,
				APIKey:         nil,
			},
		}

		factory.EXPECT().NewDownloadClient(request.DownloadClient).Return(client, nil)
		client.EXPECT().List(ctx).Return([]download.Status{}, nil)

		err := m.TestDownloadClient(ctx, request)
		assert.NoError(t, err)
	})

	t.Run("failed connection test", func(t *testing.T) {
		client := downloadMocks.NewMockDownloadClient(ctrl)
		request := AddDownloadClientRequest{
			DownloadClient: model.DownloadClient{
				Type:           "torrent",
				Implementation: "transmission",
				Scheme:         "http",
				Host:           "invalid-host",
				Port:           9091,
				APIKey:         nil,
			},
		}

		factory.EXPECT().NewDownloadClient(request.DownloadClient).Return(client, nil)
		client.EXPECT().List(ctx).Return(nil, assert.AnError)

		err := m.TestDownloadClient(ctx, request)
		assert.Error(t, err)
	})
}
