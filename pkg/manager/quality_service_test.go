package manager

import (
	"context"
	"testing"

	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/mocks"
	mediaSqlite "github.com/kasuboski/mediaz/pkg/storage/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func newQualityServiceStore(t *testing.T) storage.Storage {
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

func TestQualityService_GetQualityProfile(t *testing.T) {
	t.Run("movie quality profile", func(t *testing.T) {
		ctx := context.Background()
		qs := NewQualityService(newQualityServiceStore(t))

		profile, err := qs.GetQualityProfile(ctx, 1)
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
		qs := NewQualityService(newQualityServiceStore(t))

		profile, err := qs.GetQualityProfile(ctx, 5)
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

func TestQualityService_ListMovieQualityProfiles(t *testing.T) {
	ctx := context.Background()
	qs := NewQualityService(newQualityServiceStore(t))

	profiles, err := qs.ListMovieQualityProfiles(ctx)
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

func TestQualityService_ListEpisodeQualityProfiles(t *testing.T) {
	ctx := context.Background()
	qs := NewQualityService(newQualityServiceStore(t))

	profiles, err := qs.ListEpisodeQualityProfiles(ctx)
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

func TestQualityService_UpdateQualityProfile(t *testing.T) {
	t.Run("update profile with new quality associations", func(t *testing.T) {
		ctx := context.Background()
		qs := NewQualityService(newQualityServiceStore(t))

		profile, err := qs.GetQualityProfile(ctx, 1)
		require.NoError(t, err)
		assert.Greater(t, len(profile.Qualities), 0)

		updated, err := qs.UpdateQualityProfile(ctx, 1, UpdateQualityProfileRequest{
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
		qs := NewQualityService(newQualityServiceStore(t))

		_, err := qs.UpdateQualityProfile(ctx, 1, UpdateQualityProfileRequest{
			Name:       "Updated Profile",
			QualityIDs: []int32{},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least one quality must be selected")
	})

	t.Run("fails for non-existent profile", func(t *testing.T) {
		ctx := context.Background()
		qs := NewQualityService(newQualityServiceStore(t))

		_, err := qs.UpdateQualityProfile(ctx, 999, UpdateQualityProfileRequest{
			Name:       "Updated Profile",
			QualityIDs: []int32{3},
		})
		require.Error(t, err)
	})
}

func TestQualityService_AddQualityProfile(t *testing.T) {
	t.Run("fails when upgrade allowed but cutoff not provided", func(t *testing.T) {
		ctx := context.Background()
		qs := NewQualityService(newQualityServiceStore(t))

		_, err := qs.AddQualityProfile(ctx, AddQualityProfileRequest{
			Name:           "Test Profile",
			UpgradeAllowed: true,
			QualityIDs:     []int32{3, 7},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cutoff quality must be specified when upgrades are allowed")
	})

	t.Run("succeeds when upgrade not allowed and cutoff not provided", func(t *testing.T) {
		ctx := context.Background()
		qs := NewQualityService(newQualityServiceStore(t))

		profile, err := qs.AddQualityProfile(ctx, AddQualityProfileRequest{
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
		qs := NewQualityService(newQualityServiceStore(t))

		cutoffID := int32(10)
		_, err := qs.AddQualityProfile(ctx, AddQualityProfileRequest{
			Name:            "Test Profile",
			CutoffQualityID: &cutoffID,
			UpgradeAllowed:  true,
			QualityIDs:      []int32{3, 7},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cutoff quality must be one of the selected qualities")
	})
}

func TestMeetsQualitySize(t *testing.T) {
	tests := []struct {
		name     string
		quality  storage.QualityDefinition
		fileSize uint64
		runtime  uint64
		expected bool
	}{
		{
			name:     "within bounds",
			quality:  storage.QualityDefinition{MinSize: 10, MaxSize: 100},
			fileSize: 500,
			runtime:  10,
			expected: true,
		},
		{
			name:     "below min",
			quality:  storage.QualityDefinition{MinSize: 10, MaxSize: 100},
			fileSize: 50,
			runtime:  10,
			expected: false,
		},
		{
			name:     "above max",
			quality:  storage.QualityDefinition{MinSize: 10, MaxSize: 100},
			fileSize: 1500,
			runtime:  10,
			expected: false,
		},
		{
			name:     "at exact min",
			quality:  storage.QualityDefinition{MinSize: 10, MaxSize: 100},
			fileSize: 100,
			runtime:  10,
			expected: true,
		},
		{
			name:     "at exact max",
			quality:  storage.QualityDefinition{MinSize: 10, MaxSize: 100},
			fileSize: 1000,
			runtime:  10,
			expected: true,
		},
		{
			name:     "zero runtime",
			quality:  storage.QualityDefinition{MinSize: 10, MaxSize: 100},
			fileSize: 500,
			runtime:  0,
			expected: false,
		},
		{
			name:     "negative min size",
			quality:  storage.QualityDefinition{MinSize: -1, MaxSize: 100},
			fileSize: 500,
			runtime:  10,
			expected: false,
		},
		{
			name:     "negative max size",
			quality:  storage.QualityDefinition{MinSize: 0, MaxSize: -1},
			fileSize: 500,
			runtime:  10,
			expected: false,
		},
		{
			name:     "min greater than max",
			quality:  storage.QualityDefinition{MinSize: 100, MaxSize: 10},
			fileSize: 500,
			runtime:  10,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MeetsQualitySize(tt.quality, tt.fileSize, tt.runtime)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestQualityService_UpdateQualityProfile_Mock(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	store := mocks.NewMockStorage(ctrl)

	qs := NewQualityService(store)

	t.Run("update with new name", func(t *testing.T) {
		existingProfile := storage.QualityProfile{
			ID:              1,
			Name:            "Old Name",
			CutoffQualityID: nil,
			UpgradeAllowed:  false,
			Qualities:       []storage.QualityDefinition{},
		}

		store.EXPECT().GetQualityProfile(ctx, int64(1)).Return(existingProfile, nil)
		store.EXPECT().UpdateQualityProfile(ctx, int64(1), gomock.Any()).Return(nil)
		store.EXPECT().DeleteQualityProfileItemsByProfileID(ctx, int64(1)).Return(nil)
		store.EXPECT().CreateQualityProfileItems(ctx, gomock.Any()).Return(nil)
		store.EXPECT().GetQualityProfile(ctx, int64(1)).Return(existingProfile, nil)

		_, err := qs.UpdateQualityProfile(ctx, 1, UpdateQualityProfileRequest{
			Name:       "New Name",
			QualityIDs: []int32{1, 2},
		})
		require.NoError(t, err)
	})
}
