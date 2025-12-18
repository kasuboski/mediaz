package manager

import (
	"context"
	"fmt"
	"testing"

	"github.com/kasuboski/mediaz/pkg/storage"
	mediaSqlite "github.com/kasuboski/mediaz/pkg/storage/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQualitySizeCutoff(t *testing.T) {
	tests := []struct {
		name       string
		definition storage.QualityDefinition
		size       uint64
		runtime    uint64
		want       bool
	}{
		{
			name:    "does not meet minimum size",
			size:    1000,
			runtime: 60,
			definition: storage.QualityDefinition{
				MinSize:       17,
				MaxSize:       2000,
				PreferredSize: 1999,
			},
			want: false,
		},
		{
			name:    "meets criteria",
			size:    1026,
			runtime: 60,
			definition: storage.QualityDefinition{
				MinSize:       17.0,
				MaxSize:       2000,
				PreferredSize: 1999,
			},
			want: true,
		},

		{
			name:    "ratio too big",
			size:    120_001,
			runtime: 60,
			definition: storage.QualityDefinition{
				MinSize:       17.0,
				MaxSize:       2000,
				PreferredSize: 1999,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d,%d", tt.size, tt.runtime), func(t *testing.T) {
			if got := MeetsQualitySize(tt.definition, tt.size, tt.runtime); got != tt.want {
				t.Errorf("got %v; want %v", got, tt.want)
			}
		})
	}
}

func TestMediaManager_GetQualityProfile(t *testing.T) {
	t.Run("get movie quality profile", func(t *testing.T) {
		ctx := context.Background()
		store, err := mediaSqlite.New(context.Background(), ":memory:")
		require.NoError(t, err)

		schemas, err := storage.ReadSchemaFiles("../storage/sqlite/schema/schema.sql", "../storage/sqlite/schema/defaults.sql")
		require.NoError(t, err)
		require.NotNil(t, store)

		err = store.Init(ctx, schemas...)
		require.NoError(t, err)

		manager := MediaManager{
			storage: store,
		}

		profile, err := manager.GetQualityProfile(ctx, 1)
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

	t.Run("get episode quality profile", func(t *testing.T) {
		ctx := context.Background()
		store, err := mediaSqlite.New(context.Background(), ":memory:")
		require.NoError(t, err)

		schemas, err := storage.ReadSchemaFiles("../storage/sqlite/schema/schema.sql", "../storage/sqlite/schema/defaults.sql")
		require.NoError(t, err)
		require.NotNil(t, store)

		err = store.Init(ctx, schemas...)
		require.NoError(t, err)

		manager := MediaManager{
			storage: store,
		}

		profile, err := manager.GetQualityProfile(ctx, 5)
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

func TestMediaManager_ListMovieQualityProfiles(t *testing.T) {
	t.Run("list movie quality profiles", func(t *testing.T) {
		ctx := context.Background()
		store, err := mediaSqlite.New(context.Background(), ":memory:")
		require.NoError(t, err)

		schemas, err := storage.ReadSchemaFiles("../storage/sqlite/schema/schema.sql", "../storage/sqlite/schema/defaults.sql")
		require.NoError(t, err)
		require.NotNil(t, store)

		err = store.Init(ctx, schemas...)
		require.NoError(t, err)

		manager := MediaManager{
			storage: store,
		}

		profile, err := manager.ListMovieQualityProfiles(ctx)
		require.NoError(t, err)
		require.Len(t, profile, 3)
		assert.Equal(t, int32(3), profile[0].ID)
		assert.Equal(t, "Ultra High Definition", profile[0].Name)
		assert.Nil(t, profile[0].CutoffQualityID)
		assert.Equal(t, int32(2), profile[1].ID)
		assert.Equal(t, "High Definition", profile[1].Name)
		assert.Nil(t, profile[1].CutoffQualityID)
		assert.Equal(t, int32(2), profile[1].ID)
		assert.Equal(t, "Standard Definition", profile[2].Name)
		assert.Equal(t, int32(1), profile[2].ID)
		assert.Nil(t, profile[2].CutoffQualityID)
	})
}

func TestMediaManager_ListEpisodeQualityProfiles(t *testing.T) {
	t.Run("list episode quality profiles", func(t *testing.T) {
		ctx := context.Background()
		store, err := mediaSqlite.New(context.Background(), ":memory:")
		require.NoError(t, err)

		schemas, err := storage.ReadSchemaFiles("../storage/sqlite/schema/schema.sql", "../storage/sqlite/schema/defaults.sql")
		require.NoError(t, err)
		require.NotNil(t, store)

		err = store.Init(ctx, schemas...)
		require.NoError(t, err)

		manager := MediaManager{
			storage: store,
		}

		profile, err := manager.ListEpisodeQualityProfiles(ctx)
		require.NoError(t, err)
		require.Len(t, profile, 3)

		assert.Equal(t, "Ultra High Definition", profile[0].Name)
		assert.Equal(t, int32(6), profile[0].ID)
		assert.Nil(t, profile[0].CutoffQualityID)

		assert.Equal(t, "High Definition", profile[1].Name)
		assert.Equal(t, int32(5), profile[1].ID)
		assert.Nil(t, profile[1].CutoffQualityID)

		assert.Equal(t, "Standard Definition", profile[2].Name)
		assert.Equal(t, int32(4), profile[2].ID)
		assert.Nil(t, profile[2].CutoffQualityID)
	})
}

func TestMediaManager_UpdateQualityProfile(t *testing.T) {
	t.Run("update profile with new quality associations", func(t *testing.T) {
		ctx := context.Background()
		store, err := mediaSqlite.New(context.Background(), ":memory:")
		require.NoError(t, err)

		schemas, err := storage.ReadSchemaFiles("../storage/sqlite/schema/schema.sql", "../storage/sqlite/schema/defaults.sql")
		require.NoError(t, err)
		err = store.Init(ctx, schemas...)
		require.NoError(t, err)

		manager := MediaManager{
			storage: store,
		}

		profile, err := manager.GetQualityProfile(ctx, 1)
		require.NoError(t, err)
		initialQualityCount := len(profile.Qualities)
		assert.Greater(t, initialQualityCount, 0)

		updateReq := UpdateQualityProfileRequest{
			Name:            "Updated Profile",
			CutoffQualityID: nil,
			UpgradeAllowed:  false,
			QualityIDs:      []int32{3, 7},
		}

		updated, err := manager.UpdateQualityProfile(ctx, 1, updateReq)
		require.NoError(t, err)

		assert.Equal(t, "Updated Profile", updated.Name)
		assert.Nil(t, updated.CutoffQualityID)
		assert.Equal(t, false, updated.UpgradeAllowed)

		assert.Equal(t, 2, len(updated.Qualities))
		qualityIDs := make([]int32, len(updated.Qualities))
		for i, q := range updated.Qualities {
			qualityIDs[i] = q.ID
		}
		assert.ElementsMatch(t, []int32{3, 7}, qualityIDs)
	})

	t.Run("update fails with empty quality IDs", func(t *testing.T) {
		ctx := context.Background()
		store, err := mediaSqlite.New(context.Background(), ":memory:")
		require.NoError(t, err)

		schemas, err := storage.ReadSchemaFiles("../storage/sqlite/schema/schema.sql", "../storage/sqlite/schema/defaults.sql")
		require.NoError(t, err)
		err = store.Init(ctx, schemas...)
		require.NoError(t, err)

		manager := MediaManager{
			storage: store,
		}

		updateReq := UpdateQualityProfileRequest{
			Name:            "Updated Profile",
			CutoffQualityID: nil,
			UpgradeAllowed:  false,
			QualityIDs:      []int32{},
		}

		_, err = manager.UpdateQualityProfile(ctx, 1, updateReq)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least one quality must be selected")
	})

	t.Run("update fails for non-existent profile", func(t *testing.T) {
		ctx := context.Background()
		store, err := mediaSqlite.New(context.Background(), ":memory:")
		require.NoError(t, err)

		schemas, err := storage.ReadSchemaFiles("../storage/sqlite/schema/schema.sql", "../storage/sqlite/schema/defaults.sql")
		require.NoError(t, err)
		err = store.Init(ctx, schemas...)
		require.NoError(t, err)

		manager := MediaManager{
			storage: store,
		}

		updateReq := UpdateQualityProfileRequest{
			Name:            "Updated Profile",
			CutoffQualityID: nil,
			UpgradeAllowed:  false,
			QualityIDs:      []int32{3},
		}

		_, err = manager.UpdateQualityProfile(ctx, 999, updateReq)
		require.Error(t, err)
	})
}

func TestMediaManager_AddQualityProfile(t *testing.T) {
	t.Run("add profile fails when upgrade allowed but cutoff not provided", func(t *testing.T) {
		ctx := context.Background()
		store, err := mediaSqlite.New(context.Background(), ":memory:")
		require.NoError(t, err)

		schemas, err := storage.ReadSchemaFiles("../storage/sqlite/schema/schema.sql", "../storage/sqlite/schema/defaults.sql")
		require.NoError(t, err)
		err = store.Init(ctx, schemas...)
		require.NoError(t, err)

		manager := MediaManager{
			storage: store,
		}

		addReq := AddQualityProfileRequest{
			Name:            "Test Profile",
			CutoffQualityID: nil,
			UpgradeAllowed:  true,
			QualityIDs:      []int32{3, 7},
		}

		_, err = manager.AddQualityProfile(ctx, addReq)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cutoff quality must be specified when upgrades are allowed")
	})

	t.Run("add profile succeeds when upgrade not allowed and cutoff not provided", func(t *testing.T) {
		ctx := context.Background()
		store, err := mediaSqlite.New(context.Background(), ":memory:")
		require.NoError(t, err)

		schemas, err := storage.ReadSchemaFiles("../storage/sqlite/schema/schema.sql", "../storage/sqlite/schema/defaults.sql")
		require.NoError(t, err)
		err = store.Init(ctx, schemas...)
		require.NoError(t, err)

		manager := MediaManager{
			storage: store,
		}

		addReq := AddQualityProfileRequest{
			Name:            "Test Profile",
			CutoffQualityID: nil,
			UpgradeAllowed:  false,
			QualityIDs:      []int32{3, 7},
		}

		profile, err := manager.AddQualityProfile(ctx, addReq)
		require.NoError(t, err)
		assert.Equal(t, "Test Profile", profile.Name)
		assert.Nil(t, profile.CutoffQualityID)
		assert.Equal(t, false, profile.UpgradeAllowed)
	})

	t.Run("add profile fails when cutoff not in quality list", func(t *testing.T) {
		ctx := context.Background()
		store, err := mediaSqlite.New(context.Background(), ":memory:")
		require.NoError(t, err)

		schemas, err := storage.ReadSchemaFiles("../storage/sqlite/schema/schema.sql", "../storage/sqlite/schema/defaults.sql")
		require.NoError(t, err)
		err = store.Init(ctx, schemas...)
		require.NoError(t, err)

		manager := MediaManager{
			storage: store,
		}

		cutoffID := int32(10)
		addReq := AddQualityProfileRequest{
			Name:            "Test Profile",
			CutoffQualityID: &cutoffID,
			UpgradeAllowed:  true,
			QualityIDs:      []int32{3, 7},
		}

		_, err = manager.AddQualityProfile(ctx, addReq)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cutoff quality must be one of the selected qualities")
	})
}
