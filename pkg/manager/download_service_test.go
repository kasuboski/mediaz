package manager

import (
	"context"
	"testing"

	"github.com/kasuboski/mediaz/pkg/storage"
	mediaSqlite "github.com/kasuboski/mediaz/pkg/storage/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		assert.Equal(t, int32(1), profile.ID)
		assert.Equal(t, "Standard Definition", profile.Name)
		assert.Nil(t, profile.CutoffQualityID)
		assert.Equal(t, false, profile.UpgradeAllowed)
		assert.Equal(t, "HDTV-720p", profile.Qualities[0].Name)
		assert.Equal(t, float64(1999), profile.Qualities[0].PreferredSize)
		assert.Equal(t, float64(17.1), profile.Qualities[0].MinSize)
		assert.Equal(t, float64(2000), profile.Qualities[0].MaxSize)
		assert.Equal(t, "movie", profile.Qualities[0].MediaType)
	})

	t.Run("episode quality profile", func(t *testing.T) {
		ctx := context.Background()
		ds := NewDownloadService(nil, newDownloadServiceStore(t), nil)

		profile, err := ds.GetQualityProfile(ctx, 5)
		require.NoError(t, err)
		assert.Equal(t, int32(5), profile.ID)
		assert.Equal(t, "High Definition", profile.Name)
		assert.Nil(t, profile.CutoffQualityID)
		assert.Equal(t, false, profile.UpgradeAllowed)
		assert.Equal(t, "Remux-1080p", profile.Qualities[0].Name)
		assert.Equal(t, float64(995), profile.Qualities[0].PreferredSize)
		assert.Equal(t, float64(69.1), profile.Qualities[0].MinSize)
		assert.Equal(t, float64(1000), profile.Qualities[0].MaxSize)
		assert.Equal(t, "episode", profile.Qualities[0].MediaType)
	})
}

func TestDownloadService_ListMovieQualityProfiles(t *testing.T) {
	ctx := context.Background()
	ds := NewDownloadService(nil, newDownloadServiceStore(t), nil)

	profiles, err := ds.ListMovieQualityProfiles(ctx)
	require.NoError(t, err)
	require.Len(t, profiles, 3)
	assert.Equal(t, int32(3), profiles[0].ID)
	assert.Equal(t, "Ultra High Definition", profiles[0].Name)
	assert.Nil(t, profiles[0].CutoffQualityID)
	assert.Equal(t, int32(2), profiles[1].ID)
	assert.Equal(t, "High Definition", profiles[1].Name)
	assert.Nil(t, profiles[1].CutoffQualityID)
	assert.Equal(t, "Standard Definition", profiles[2].Name)
	assert.Equal(t, int32(1), profiles[2].ID)
	assert.Nil(t, profiles[2].CutoffQualityID)
}

func TestDownloadService_ListEpisodeQualityProfiles(t *testing.T) {
	ctx := context.Background()
	ds := NewDownloadService(nil, newDownloadServiceStore(t), nil)

	profiles, err := ds.ListEpisodeQualityProfiles(ctx)
	require.NoError(t, err)
	require.Len(t, profiles, 3)
	assert.Equal(t, "Ultra High Definition", profiles[0].Name)
	assert.Equal(t, int32(6), profiles[0].ID)
	assert.Nil(t, profiles[0].CutoffQualityID)
	assert.Equal(t, "High Definition", profiles[1].Name)
	assert.Equal(t, int32(5), profiles[1].ID)
	assert.Nil(t, profiles[1].CutoffQualityID)
	assert.Equal(t, "Standard Definition", profiles[2].Name)
	assert.Equal(t, int32(4), profiles[2].ID)
	assert.Nil(t, profiles[2].CutoffQualityID)
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
		assert.Equal(t, "Updated Profile", updated.Name)
		assert.Nil(t, updated.CutoffQualityID)
		assert.Equal(t, false, updated.UpgradeAllowed)
		require.Len(t, updated.Qualities, 2)
		qualityIDs := make([]int32, len(updated.Qualities))
		for i, q := range updated.Qualities {
			qualityIDs[i] = q.ID
		}
		assert.ElementsMatch(t, []int32{3, 7}, qualityIDs)
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
		assert.Equal(t, "Test Profile", profile.Name)
		assert.Nil(t, profile.CutoffQualityID)
		assert.Equal(t, false, profile.UpgradeAllowed)
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
