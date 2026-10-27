package manager

import (
	"context"
	"database/sql"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
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
	return store
}

func TestQualityService_GetQualityProfile(t *testing.T) {
	t.Run("movie quality profile", func(t *testing.T) {
		ctx := context.Background()
		qs := NewQualityService(newQualityServiceStore(t))

		profile, err := qs.GetQualityProfile(ctx, 1)
		require.NoError(t, err)
		want := sqlcdb.QualityProfile{
			ID:              1,
			Name:            "Standard Definition",
			CutoffQualityID: sql.NullInt64{Int64: 2, Valid: true},
			UpgradeAllowed:  true,
		}
		assert.Equal(t, want, profile)
	})

	t.Run("episode quality profile", func(t *testing.T) {
		ctx := context.Background()
		qs := NewQualityService(newQualityServiceStore(t))

		profile, err := qs.GetQualityProfile(ctx, 5)
		require.NoError(t, err)
		want := sqlcdb.QualityProfile{
			ID:              5,
			Name:            "High Definition",
			CutoffQualityID: sql.NullInt64{Int64: 23, Valid: true},
			UpgradeAllowed:  true,
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

	want := []sqlcdb.QualityProfile{
		{
			ID: 3, Name: "Ultra High Definition", CutoffQualityID: sql.NullInt64{Int64: 13, Valid: true}, UpgradeAllowed: false,
		},
		{
			ID: 2, Name: "High Definition", CutoffQualityID: sql.NullInt64{Int64: 8, Valid: true}, UpgradeAllowed: true,
		},
		{
			ID: 1, Name: "Standard Definition", CutoffQualityID: sql.NullInt64{Int64: 2, Valid: true}, UpgradeAllowed: true,
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

	want := []sqlcdb.QualityProfile{
		{
			ID: 6, Name: "Ultra High Definition", CutoffQualityID: sql.NullInt64{Int64: 27, Valid: true}, UpgradeAllowed: false,
		},
		{
			ID: 5, Name: "High Definition", CutoffQualityID: sql.NullInt64{Int64: 23, Valid: true}, UpgradeAllowed: true,
		},
		{
			ID: 4, Name: "Standard Definition", CutoffQualityID: sql.NullInt64{Int64: 16, Valid: true}, UpgradeAllowed: true,
		},
	}
	assert.Equal(t, want, profiles)
}

func TestQualityService_UpdateQualityProfile(t *testing.T) {
	t.Run("update profile with new quality associations", func(t *testing.T) {
		ctx := context.Background()
		qs := NewQualityService(newQualityServiceStore(t))

		rows, err := qs.ListQualityProfileRows(ctx, 1)
		require.NoError(t, err)
		assert.NotEmpty(t, rows)

		updated, err := qs.UpdateQualityProfile(ctx, 1, UpdateQualityProfileRequest{
			Name:           "Updated Profile",
			UpgradeAllowed: false,
			QualityIDs:     []int64{3, 7},
		})
		require.NoError(t, err)
		want := sqlcdb.QualityProfile{
			ID:              1,
			Name:            "Updated Profile",
			CutoffQualityID: sql.NullInt64{},
			UpgradeAllowed:  false,
		}
		assert.Equal(t, want, updated)
	})

	t.Run("fails with empty quality IDs", func(t *testing.T) {
		ctx := context.Background()
		qs := NewQualityService(newQualityServiceStore(t))

		_, err := qs.UpdateQualityProfile(ctx, 1, UpdateQualityProfileRequest{
			Name:       "Updated Profile",
			QualityIDs: []int64{},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least one quality must be selected")
	})

	t.Run("fails for non-existent profile", func(t *testing.T) {
		ctx := context.Background()
		qs := NewQualityService(newQualityServiceStore(t))

		_, err := qs.UpdateQualityProfile(ctx, 999, UpdateQualityProfileRequest{
			Name:       "Updated Profile",
			QualityIDs: []int64{3},
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
			QualityIDs:     []int64{3, 7},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cutoff quality must be specified when upgrades are allowed")
	})

	t.Run("succeeds when upgrade not allowed and cutoff not provided", func(t *testing.T) {
		ctx := context.Background()
		qs := NewQualityService(newQualityServiceStore(t))

		profile, err := qs.AddQualityProfile(ctx, AddQualityProfileRequest{
			Name:       "Test Profile",
			QualityIDs: []int64{3, 7},
		})
		require.NoError(t, err)
		want := sqlcdb.QualityProfile{
			ID:              profile.ID,
			Name:            "Test Profile",
			CutoffQualityID: sql.NullInt64{},
			UpgradeAllowed:  false,
		}
		assert.Equal(t, want, profile)
	})

	t.Run("fails when cutoff not in quality list", func(t *testing.T) {
		ctx := context.Background()
		qs := NewQualityService(newQualityServiceStore(t))

		cutoffID := int64(10)
		_, err := qs.AddQualityProfile(ctx, AddQualityProfileRequest{
			Name:            "Test Profile",
			CutoffQualityID: &cutoffID,
			UpgradeAllowed:  true,
			QualityIDs:      []int64{3, 7},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cutoff quality must be one of the selected qualities")
	})
}

func TestMeetsQualitySize(t *testing.T) {
	tests := []struct {
		name     string
		quality  sqlcdb.QualityDefinition
		fileSize uint64
		runtime  uint64
		expected bool
	}{
		{
			name:     "within bounds",
			quality:  sqlcdb.QualityDefinition{MinSize: 10, MaxSize: 100},
			fileSize: 500,
			runtime:  10,
			expected: true,
		},
		{
			name:     "below min",
			quality:  sqlcdb.QualityDefinition{MinSize: 10, MaxSize: 100},
			fileSize: 50,
			runtime:  10,
			expected: false,
		},
		{
			name:     "above max",
			quality:  sqlcdb.QualityDefinition{MinSize: 10, MaxSize: 100},
			fileSize: 1500,
			runtime:  10,
			expected: false,
		},
		{
			name:     "at exact min",
			quality:  sqlcdb.QualityDefinition{MinSize: 10, MaxSize: 100},
			fileSize: 100,
			runtime:  10,
			expected: true,
		},
		{
			name:     "at exact max",
			quality:  sqlcdb.QualityDefinition{MinSize: 10, MaxSize: 100},
			fileSize: 1000,
			runtime:  10,
			expected: true,
		},
		{
			name:     "zero runtime",
			quality:  sqlcdb.QualityDefinition{MinSize: 10, MaxSize: 100},
			fileSize: 500,
			runtime:  0,
			expected: false,
		},
		{
			name:     "negative min size",
			quality:  sqlcdb.QualityDefinition{MinSize: -1, MaxSize: 100},
			fileSize: 500,
			runtime:  10,
			expected: false,
		},
		{
			name:     "negative max size",
			quality:  sqlcdb.QualityDefinition{MinSize: 0, MaxSize: -1},
			fileSize: 500,
			runtime:  10,
			expected: false,
		},
		{
			name:     "min greater than max",
			quality:  sqlcdb.QualityDefinition{MinSize: 100, MaxSize: 10},
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
		existingProfile := sqlcdb.QualityProfile{
			ID:              1,
			Name:            "Old Name",
			CutoffQualityID: sql.NullInt64{Int64: 2, Valid: true},
			UpgradeAllowed:  true,
		}

		store.EXPECT().GetQualityProfile(ctx, int64(1)).Return(existingProfile, nil)
		store.EXPECT().UpdateQualityProfile(ctx, int64(1), gomock.Any()).Return(nil)
		store.EXPECT().DeleteQualityProfileItemsByProfileID(ctx, int64(1)).Return(nil)
		store.EXPECT().CreateQualityProfileItems(ctx, gomock.Any()).Return(nil)
		store.EXPECT().GetQualityProfile(ctx, int64(1)).Return(existingProfile, nil)

		_, err := qs.UpdateQualityProfile(ctx, 1, UpdateQualityProfileRequest{
			Name:       "New Name",
			QualityIDs: []int64{1, 2},
		})
		require.NoError(t, err)
	})
}
