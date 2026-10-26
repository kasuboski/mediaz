package sqlite

import (
	"context"
	"testing"

	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestGetQualityProfileNotFound(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)

	_, err := store.GetQualityProfile(ctx, 1234)
	assert.ErrorIs(t, err, storage.ErrNotFound)
}

func TestGetQualityProfileOrdersQualitiesByMinSizeDesc(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)

	profileID, err := store.CreateQualityProfile(ctx, model.QualityProfile{
		Name:           "ordered",
		UpgradeAllowed: false,
	})
	require.NoError(t, err)

	smallID, err := store.CreateQualityDefinition(ctx, model.QualityDefinition{
		QualityID: 1, Name: "small", PreferredSize: 100, MinSize: 5, MaxSize: 100, MediaType: "movie",
	})
	require.NoError(t, err)
	largeID, err := store.CreateQualityDefinition(ctx, model.QualityDefinition{
		QualityID: 2, Name: "large", PreferredSize: 200, MinSize: 50, MaxSize: 200, MediaType: "movie",
	})
	require.NoError(t, err)

	// Insert out of order on purpose; the join query orders by min size.
	for _, qualityID := range []int64{smallID, largeID} {
		_, err := store.CreateQualityProfileItem(ctx, model.QualityProfileItem{
			ProfileID: int32(profileID),
			QualityID: int32(qualityID),
		})
		require.NoError(t, err)
	}

	profile, err := store.GetQualityProfile(ctx, profileID)
	require.NoError(t, err)
	require.Len(t, profile.Qualities, 2)
	assert.Equal(t, "large", profile.Qualities[0].Name)
	assert.Equal(t, "small", profile.Qualities[1].Name)
}

func TestQualityProfileLists(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)

	// Migrations seed six default profiles (three movie, three episode).
	baselineAll, err := store.ListQualityProfiles(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, baselineAll)
	baselineMovies, err := store.ListQualityProfilesByMediaType(ctx, "movie")
	require.NoError(t, err)
	require.NotEmpty(t, baselineMovies)
	baselineEpisodes, err := store.ListQualityProfilesByMediaType(ctx, "episode")
	require.NoError(t, err)
	require.NotEmpty(t, baselineEpisodes)

	movieDefID, err := store.CreateQualityDefinition(ctx, model.QualityDefinition{
		QualityID: 101, Name: "movie quality", PreferredSize: 100, MinSize: 10, MaxSize: 200, MediaType: "movie",
	})
	require.NoError(t, err)
	episodeDefID, err := store.CreateQualityDefinition(ctx, model.QualityDefinition{
		QualityID: 102, Name: "episode quality", PreferredSize: 100, MinSize: 20, MaxSize: 200, MediaType: "episode",
	})
	require.NoError(t, err)

	movieProfileID, err := store.CreateQualityProfile(ctx, model.QualityProfile{Name: "movie profile"})
	require.NoError(t, err)
	_, err = store.CreateQualityProfileItem(ctx, model.QualityProfileItem{
		ProfileID: int32(movieProfileID), QualityID: int32(movieDefID),
	})
	require.NoError(t, err)

	episodeProfileID, err := store.CreateQualityProfile(ctx, model.QualityProfile{Name: "episode profile"})
	require.NoError(t, err)
	_, err = store.CreateQualityProfileItem(ctx, model.QualityProfileItem{
		ProfileID: int32(episodeProfileID), QualityID: int32(episodeDefID),
	})
	require.NoError(t, err)

	findByName := func(profiles []*storage.QualityProfile, name string) *storage.QualityProfile {
		for _, p := range profiles {
			if p.Name == name {
				return p
			}
		}
		return nil
	}

	all, err := store.ListQualityProfiles(ctx)
	require.NoError(t, err)
	require.Len(t, all, len(baselineAll)+2)
	// Profiles are ordered by their largest quality min size; the episode
	// profile (min 20) must appear before the movie profile (min 10).
	episodeIdx, movieIdx := -1, -1
	for i, p := range all {
		switch p.Name {
		case "episode profile":
			episodeIdx = i
		case "movie profile":
			movieIdx = i
		}
	}
	require.NotEqual(t, -1, episodeIdx)
	require.NotEqual(t, -1, movieIdx)
	assert.Less(t, episodeIdx, movieIdx)

	movies, err := store.ListQualityProfilesByMediaType(ctx, "movie")
	require.NoError(t, err)
	require.Len(t, movies, len(baselineMovies)+1)
	movie := findByName(movies, "movie profile")
	require.NotNil(t, movie)
	require.Len(t, movie.Qualities, 1)
	assert.Equal(t, int32(movieDefID), movie.Qualities[0].ID)
	assert.Equal(t, int32(movieProfileID), movie.ID)

	episodes, err := store.ListQualityProfilesByMediaType(ctx, "episode")
	require.NoError(t, err)
	require.Len(t, episodes, len(baselineEpisodes)+1)
	assert.NotNil(t, findByName(episodes, "episode profile"))

	none, err := store.ListQualityProfilesByMediaType(ctx, "audiobook")
	require.NoError(t, err)
	assert.Empty(t, none)
}

func TestUpdateQualityProfile(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)

	cutoff := int32(1)
	profileID, err := store.CreateQualityProfile(ctx, model.QualityProfile{
		Name:           "before",
		UpgradeAllowed: false,
	})
	require.NoError(t, err)

	err = store.UpdateQualityProfile(ctx, profileID, storage.QualityProfile{
		ID:              int32(profileID),
		Name:            "after",
		CutoffQualityID: &cutoff,
		UpgradeAllowed:  true,
	})
	require.NoError(t, err)

	profile, err := store.GetQualityProfile(ctx, profileID)
	require.ErrorIs(t, err, storage.ErrNotFound) // no items linked yet, so the join yields no rows

	// Verify the updated profile row once an item exists.
	defID, err := store.CreateQualityDefinition(ctx, model.QualityDefinition{
		QualityID: 1, Name: "q", PreferredSize: 100, MinSize: 10, MaxSize: 200, MediaType: "movie",
	})
	require.NoError(t, err)
	_, err = store.CreateQualityProfileItem(ctx, model.QualityProfileItem{
		ProfileID: int32(profileID), QualityID: int32(defID),
	})
	require.NoError(t, err)

	profile, err = store.GetQualityProfile(ctx, profileID)
	require.NoError(t, err)
	assert.Equal(t, "after", profile.Name)
	assert.True(t, profile.UpgradeAllowed)
	require.NotNil(t, profile.CutoffQualityID)
	assert.Equal(t, cutoff, *profile.CutoffQualityID)
}

func TestDeleteQualityProfileRemovesItems(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)

	baselineItems, err := store.ListQualityProfileItems(ctx)
	require.NoError(t, err)
	baselineProfiles, err := store.ListQualityProfiles(ctx)
	require.NoError(t, err)

	profileID, err := store.CreateQualityProfile(ctx, model.QualityProfile{Name: "doomed"})
	require.NoError(t, err)
	defID, err := store.CreateQualityDefinition(ctx, model.QualityDefinition{
		QualityID: 1, Name: "q", PreferredSize: 100, MinSize: 10, MaxSize: 200, MediaType: "movie",
	})
	require.NoError(t, err)
	_, err = store.CreateQualityProfileItem(ctx, model.QualityProfileItem{
		ProfileID: int32(profileID), QualityID: int32(defID),
	})
	require.NoError(t, err)

	err = store.DeleteQualityProfile(ctx, profileID)
	require.NoError(t, err)

	items, err := store.ListQualityProfileItems(ctx)
	require.NoError(t, err)
	assert.Len(t, items, len(baselineItems))

	profiles, err := store.ListQualityProfiles(ctx)
	require.NoError(t, err)
	assert.Len(t, profiles, len(baselineProfiles))
	for _, p := range profiles {
		assert.NotEqual(t, "doomed", p.Name)
	}
}

func TestQualityProfileItemBatchOperations(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)

	baseline, err := store.ListQualityProfileItems(ctx)
	require.NoError(t, err)

	profileID, err := store.CreateQualityProfile(ctx, model.QualityProfile{Name: "batch"})
	require.NoError(t, err)
	def1ID, err := store.CreateQualityDefinition(ctx, model.QualityDefinition{
		QualityID: 201, Name: "q1", PreferredSize: 100, MinSize: 10, MaxSize: 200, MediaType: "movie",
	})
	require.NoError(t, err)
	def2ID, err := store.CreateQualityDefinition(ctx, model.QualityDefinition{
		QualityID: 202, Name: "q2", PreferredSize: 100, MinSize: 15, MaxSize: 200, MediaType: "movie",
	})
	require.NoError(t, err)

	// Empty batch is a no-op.
	err = store.CreateQualityProfileItems(ctx, nil)
	require.NoError(t, err)

	err = store.CreateQualityProfileItems(ctx, []model.QualityProfileItem{
		{ProfileID: int32(profileID), QualityID: int32(def1ID)},
		{ProfileID: int32(profileID), QualityID: int32(def2ID)},
	})
	require.NoError(t, err)

	items, err := store.ListQualityProfileItems(ctx)
	require.NoError(t, err)
	require.Len(t, items, len(baseline)+2)

	var mine []*model.QualityProfileItem
	for _, item := range items {
		if item.ProfileID == int32(profileID) {
			mine = append(mine, item)
		}
	}
	require.Len(t, mine, 2)
	for _, item := range mine {
		require.NotNil(t, item.ID)
		assert.Positive(t, *item.ID)
	}
	assert.ElementsMatch(t, []int32{int32(def1ID), int32(def2ID)},
		[]int32{mine[0].QualityID, mine[1].QualityID})

	err = store.DeleteQualityProfileItemsByProfileID(ctx, profileID)
	require.NoError(t, err)

	items, err = store.ListQualityProfileItems(ctx)
	require.NoError(t, err)
	assert.Len(t, items, len(baseline))
	for _, item := range items {
		assert.NotEqual(t, int32(profileID), item.ProfileID)
	}
}

func TestGetQualityProfileItemNotFound(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)

	_, err := store.GetQualityProfileItem(ctx, 1234)
	assert.ErrorIs(t, err, storage.ErrNotFound)
}
