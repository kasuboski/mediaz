package manager

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQualityServiceMissingCutoffDefinitionPreservesProfilesAndItems(t *testing.T) {
	t.Run("add", func(t *testing.T) {
		ctx := context.Background()
		store := newQualityServiceStore(t)
		service := NewQualityService(store)
		missingID := int64(999999)
		_, err := service.GetQualityDefinition(ctx, missingID)
		require.Error(t, err)

		profilesBefore, err := store.ListQualityProfiles(ctx)
		require.NoError(t, err)
		itemsBefore, err := store.ListQualityProfileItems(ctx)
		require.NoError(t, err)

		// Include the missing cutoff among the selections so request validation
		// succeeds and SQLite must reject the nonexistent definition.
		_, err = service.AddQualityProfile(ctx, AddQualityProfileRequest{
			Name:            "Missing cutoff definition",
			CutoffQualityID: &missingID,
			UpgradeAllowed:  true,
			QualityIDs:      []int64{3, missingID},
		})
		require.Error(t, err)

		profilesAfter, err := store.ListQualityProfiles(ctx)
		require.NoError(t, err)
		itemsAfter, err := store.ListQualityProfileItems(ctx)
		require.NoError(t, err)
		assert.Equal(t, profilesBefore, profilesAfter)
		assert.Equal(t, itemsBefore, itemsAfter)
	})

	t.Run("update", func(t *testing.T) {
		ctx := context.Background()
		store := newQualityServiceStore(t)
		service := NewQualityService(store)
		cutoffID := int64(7)
		profile, err := service.AddQualityProfile(ctx, AddQualityProfileRequest{
			Name:            "Original profile",
			CutoffQualityID: &cutoffID,
			UpgradeAllowed:  true,
			QualityIDs:      []int64{3, 7},
		})
		require.NoError(t, err)
		missingID := int64(999999)
		_, err = service.GetQualityDefinition(ctx, missingID)
		require.Error(t, err)

		profilesBefore, err := store.ListQualityProfiles(ctx)
		require.NoError(t, err)
		itemsBefore, err := store.ListQualityProfileItems(ctx)
		require.NoError(t, err)

		_, err = service.UpdateQualityProfile(ctx, profile.ID, UpdateQualityProfileRequest{
			Name:            "Invalid replacement",
			CutoffQualityID: &missingID,
			UpgradeAllowed:  false,
			QualityIDs:      []int64{3, missingID},
		})
		require.Error(t, err)

		profilesAfter, err := store.ListQualityProfiles(ctx)
		require.NoError(t, err)
		itemsAfter, err := store.ListQualityProfileItems(ctx)
		require.NoError(t, err)
		assert.Equal(t, profilesBefore, profilesAfter)
		assert.Equal(t, itemsBefore, itemsAfter)
	})
}
