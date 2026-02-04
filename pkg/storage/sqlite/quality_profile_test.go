package sqlite

import (
	"context"
	"testing"

	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/stretchr/testify/assert"
)

func TestGetQualityStorage(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)

	cutoffID := int32(3)
	testProfile := model.QualityProfile{
		Name:            "test profile",
		UpgradeAllowed:  true,
		CutoffQualityID: &cutoffID,
	}

	profileID, err := store.CreateQualityProfile(ctx, testProfile)
	assert.Nil(t, err)
	assert.NotZero(t, profileID)

	firstDefinition := model.QualityDefinition{
		Name:          "test quality definition 1",
		PreferredSize: 1999,
		MinSize:       15,
		MaxSize:       2000,
		MediaType:     "movie",
	}
	def1ID, err := store.CreateQualityDefinition(ctx, firstDefinition)
	assert.Nil(t, err)
	assert.NotZero(t, def1ID)

	definitionOne, err := store.GetQualityDefinition(ctx, def1ID)
	assert.Nil(t, err)
	firstDefinition.ID = int32(def1ID)
	assert.Equal(t, firstDefinition, definitionOne)

	secondDefinition := model.QualityDefinition{
		Name:          "test quality definition 2",
		PreferredSize: 1499,
		MinSize:       10,
		MaxSize:       1500,
		MediaType:     "movie",
	}
	def2ID, err := store.CreateQualityDefinition(ctx, secondDefinition)
	assert.Nil(t, err)
	assert.NotZero(t, def2ID)

	definitionTwo, err := store.GetQualityDefinition(ctx, def2ID)
	assert.Nil(t, err)
	secondDefinition.ID = int32(def2ID)
	assert.Equal(t, secondDefinition, definitionTwo)

	firstQualityItem := model.QualityProfileItem{
		ProfileID: int32(profileID),
		QualityID: int32(def1ID),
	}
	item1ID, err := store.CreateQualityProfileItem(ctx, firstQualityItem)
	assert.Nil(t, err)
	assert.NotZero(t, item1ID)

	firstItem, err := store.GetQualityProfileItem(ctx, item1ID)
	assert.Nil(t, err)
	i32ID := int32(item1ID)
	firstQualityItem.ID = &i32ID
	assert.Equal(t, firstQualityItem, firstItem)

	secondQualityItem := model.QualityProfileItem{
		ProfileID: int32(profileID),
		QualityID: int32(def2ID),
	}
	item2ID, err := store.CreateQualityProfileItem(ctx, secondQualityItem)
	assert.Nil(t, err)
	assert.NotZero(t, item2ID)

	secondItem, err := store.GetQualityProfileItem(ctx, item2ID)
	assert.Nil(t, err)
	i32ID = int32(item2ID)
	secondQualityItem.ID = &i32ID
	assert.Equal(t, secondQualityItem, secondItem)

	profile, err := store.GetQualityProfile(ctx, profileID)
	assert.Nil(t, err)
	assert.Equal(t, "test profile", profile.Name)
	assert.Equal(t, true, profile.UpgradeAllowed)
	assert.Equal(t, int32(3), *profile.CutoffQualityID)
	assert.ElementsMatch(t, []storage.QualityDefinition{
		{
			ID:            int32(def1ID),
			Name:          "test quality definition 1",
			PreferredSize: 1999,
			MinSize:       15,
			MaxSize:       2000,
			MediaType:     "movie",
		},
		{
			ID:            int32(def2ID),
			Name:          "test quality definition 2",
			PreferredSize: 1499,
			MinSize:       10,
			MaxSize:       1500,
			MediaType:     "movie",
		},
	}, profile.Qualities)

	err = store.DeleteQualityDefinition(ctx, def1ID)
	assert.Nil(t, err)

	err = store.DeleteQualityDefinition(ctx, def2ID)
	assert.Nil(t, err)

	err = store.DeleteQualityProfileItem(ctx, item1ID)
	assert.Nil(t, err)

	err = store.DeleteQualityProfileItem(ctx, item2ID)
	assert.Nil(t, err)

	err = store.DeleteQualityProfile(ctx, profileID)
	assert.Nil(t, err)
}
