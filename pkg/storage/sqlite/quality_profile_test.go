package sqlite

import (
	"context"
	"database/sql"
	"testing"

	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetQualityStorage(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)

	cutoffID := int64(3)
	testProfile := sqlcdb.QualityProfile{
		Name:            "test profile",
		UpgradeAllowed:  true,
		CutoffQualityID: sql.NullInt64{Int64: cutoffID, Valid: true},
	}

	profileID, err := store.CreateQualityProfile(ctx, testProfile)
	assert.Nil(t, err)
	assert.NotZero(t, profileID)

	firstDefinition := sqlcdb.QualityDefinition{
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
	firstDefinition.ID = int64(def1ID)
	assert.Equal(t, firstDefinition, definitionOne)

	secondDefinition := sqlcdb.QualityDefinition{
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
	secondDefinition.ID = int64(def2ID)
	assert.Equal(t, secondDefinition, definitionTwo)

	firstQualityItem := sqlcdb.QualityProfileItem{
		ProfileID: int64(profileID),
		QualityID: int64(def1ID),
	}
	item1ID, err := store.CreateQualityProfileItem(ctx, firstQualityItem)
	assert.Nil(t, err)
	assert.NotZero(t, item1ID)

	firstItem, err := store.GetQualityProfileItem(ctx, item1ID)
	assert.Nil(t, err)
	firstQualityItem.ID = item1ID
	assert.Equal(t, firstQualityItem, firstItem)

	secondQualityItem := sqlcdb.QualityProfileItem{
		ProfileID: int64(profileID),
		QualityID: int64(def2ID),
	}
	item2ID, err := store.CreateQualityProfileItem(ctx, secondQualityItem)
	assert.Nil(t, err)
	assert.NotZero(t, item2ID)

	secondItem, err := store.GetQualityProfileItem(ctx, item2ID)
	assert.Nil(t, err)
	secondQualityItem.ID = item2ID
	assert.Equal(t, secondQualityItem, secondItem)

	profile, err := store.GetQualityProfile(ctx, profileID)
	assert.Nil(t, err)
	testProfile.ID = profileID
	assert.Equal(t, testProfile, profile)
	rows, err := store.ListQualityProfileRows(ctx, profileID)
	require.NoError(t, err)
	assert.Equal(t, []sqlcdb.ListQualityProfileRowsRow{
		{QualityProfile: testProfile, QualityDefinition: firstDefinition},
		{QualityProfile: testProfile, QualityDefinition: secondDefinition},
	}, rows)

	// Definitions cannot be deleted while profile items reference them.
	err = store.DeleteQualityDefinition(ctx, def1ID)
	require.ErrorContains(t, err, "FOREIGN KEY")

	err = store.DeleteQualityProfileItem(ctx, item1ID)
	require.NoError(t, err)
	err = store.DeleteQualityProfileItem(ctx, item2ID)
	require.NoError(t, err)
	err = store.DeleteQualityDefinition(ctx, def1ID)
	require.NoError(t, err)
	err = store.DeleteQualityDefinition(ctx, def2ID)
	require.NoError(t, err)

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

	profileID, err := store.CreateQualityProfile(ctx, sqlcdb.QualityProfile{
		Name:           "ordered",
		UpgradeAllowed: false,
	})
	require.NoError(t, err)

	smallID, err := store.CreateQualityDefinition(ctx, sqlcdb.QualityDefinition{
		QualityID: 1, Name: "small", PreferredSize: 100, MinSize: 5, MaxSize: 100, MediaType: "movie",
	})
	require.NoError(t, err)
	largeID, err := store.CreateQualityDefinition(ctx, sqlcdb.QualityDefinition{
		QualityID: 2, Name: "large", PreferredSize: 200, MinSize: 50, MaxSize: 200, MediaType: "movie",
	})
	require.NoError(t, err)

	// Insert out of order on purpose; the join query orders by min size.
	for _, qualityID := range []int64{smallID, largeID} {
		_, err := store.CreateQualityProfileItem(ctx, sqlcdb.QualityProfileItem{
			ProfileID: int64(profileID),
			QualityID: int64(qualityID),
		})
		require.NoError(t, err)
	}

	profile, err := store.ListQualityProfileRows(ctx, profileID)
	require.NoError(t, err)
	require.Len(t, profile, 2)
	assert.Equal(t, []sqlcdb.ListQualityProfileRowsRow{
		{QualityProfile: sqlcdb.QualityProfile{ID: profileID, Name: "ordered"}, QualityDefinition: sqlcdb.QualityDefinition{ID: largeID, QualityID: 2, Name: "large", PreferredSize: 200, MinSize: 50, MaxSize: 200, MediaType: "movie"}},
		{QualityProfile: sqlcdb.QualityProfile{ID: profileID, Name: "ordered"}, QualityDefinition: sqlcdb.QualityDefinition{ID: smallID, QualityID: 1, Name: "small", PreferredSize: 100, MinSize: 5, MaxSize: 100, MediaType: "movie"}},
	}, profile)
}

func TestQualityProfileLists(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)

	// Bootstrap seeds six default profiles (three movie, three episode).
	// Entity list APIs return each profile once, independently of its qualities.
	baselineAll, err := store.ListQualityProfiles(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, baselineAll)
	baselineMovies, err := store.ListQualityProfilesByMediaType(ctx, "movie")
	require.NoError(t, err)
	require.NotEmpty(t, baselineMovies)
	baselineEpisodes, err := store.ListQualityProfilesByMediaType(ctx, "episode")
	require.NoError(t, err)
	require.NotEmpty(t, baselineEpisodes)

	movieDefID, err := store.CreateQualityDefinition(ctx, sqlcdb.QualityDefinition{
		QualityID: 101, Name: "movie quality", PreferredSize: 100, MinSize: 10, MaxSize: 200, MediaType: "movie",
	})
	require.NoError(t, err)
	episodeDefID, err := store.CreateQualityDefinition(ctx, sqlcdb.QualityDefinition{
		QualityID: 102, Name: "episode quality", PreferredSize: 100, MinSize: 20, MaxSize: 200, MediaType: "episode",
	})
	require.NoError(t, err)

	movieProfileID, err := store.CreateQualityProfile(ctx, sqlcdb.QualityProfile{Name: "movie profile"})
	require.NoError(t, err)
	_, err = store.CreateQualityProfileItem(ctx, sqlcdb.QualityProfileItem{
		ProfileID: int64(movieProfileID), QualityID: int64(movieDefID),
	})
	require.NoError(t, err)

	episodeProfileID, err := store.CreateQualityProfile(ctx, sqlcdb.QualityProfile{Name: "episode profile"})
	require.NoError(t, err)
	_, err = store.CreateQualityProfileItem(ctx, sqlcdb.QualityProfileItem{
		ProfileID: int64(episodeProfileID), QualityID: int64(episodeDefID),
	})
	require.NoError(t, err)

	movieProfile := sqlcdb.QualityProfile{ID: movieProfileID, Name: "movie profile"}
	episodeProfile := sqlcdb.QualityProfile{ID: episodeProfileID, Name: "episode profile"}
	all, err := store.ListQualityProfiles(ctx)
	require.NoError(t, err)
	assert.ElementsMatch(t, append(baselineAll, movieProfile, episodeProfile), all)
	movies, err := store.ListQualityProfilesByMediaType(ctx, "movie")
	require.NoError(t, err)
	assert.ElementsMatch(t, append(baselineMovies, movieProfile), movies)
	episodes, err := store.ListQualityProfilesByMediaType(ctx, "episode")
	require.NoError(t, err)
	assert.ElementsMatch(t, append(baselineEpisodes, episodeProfile), episodes)

	rows, err := store.ListQualityProfileRows(ctx, 0)
	require.NoError(t, err)
	assert.Contains(t, rows, sqlcdb.ListQualityProfileRowsRow{
		QualityProfile:    movieProfile,
		QualityDefinition: sqlcdb.QualityDefinition{ID: movieDefID, QualityID: 101, Name: "movie quality", PreferredSize: 100, MinSize: 10, MaxSize: 200, MediaType: "movie"},
	})
	assert.Contains(t, rows, sqlcdb.ListQualityProfileRowsRow{
		QualityProfile:    episodeProfile,
		QualityDefinition: sqlcdb.QualityDefinition{ID: episodeDefID, QualityID: 102, Name: "episode quality", PreferredSize: 100, MinSize: 20, MaxSize: 200, MediaType: "episode"},
	})

	none, err := store.ListQualityProfilesByMediaType(ctx, "audiobook")
	require.NoError(t, err)
	assert.Empty(t, none)
}

func TestUpdateQualityProfile(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)

	cutoff := int64(1)
	profileID, err := store.CreateQualityProfile(ctx, sqlcdb.QualityProfile{
		Name:           "before",
		UpgradeAllowed: false,
	})
	require.NoError(t, err)

	err = store.UpdateQualityProfile(ctx, profileID, sqlcdb.QualityProfile{
		ID:              int64(profileID),
		Name:            "after",
		CutoffQualityID: sql.NullInt64{Int64: cutoff, Valid: true},
		UpgradeAllowed:  true,
	})
	require.NoError(t, err)

	profile, err := store.GetQualityProfile(ctx, profileID)
	require.NoError(t, err)
	expectedProfile := sqlcdb.QualityProfile{ID: profileID, Name: "after", UpgradeAllowed: true, CutoffQualityID: sql.NullInt64{Int64: cutoff, Valid: true}}
	assert.Equal(t, expectedProfile, profile)
	profiles, err := store.ListQualityProfiles(ctx)
	require.NoError(t, err)
	assert.Contains(t, profiles, expectedProfile)
	rows, err := store.ListQualityProfileRows(ctx, profileID)
	require.NoError(t, err)
	assert.Empty(t, rows)

	// Verify the updated profile row once an item exists.
	defID, err := store.CreateQualityDefinition(ctx, sqlcdb.QualityDefinition{
		QualityID: 1, Name: "q", PreferredSize: 100, MinSize: 10, MaxSize: 200, MediaType: "movie",
	})
	require.NoError(t, err)
	_, err = store.CreateQualityProfileItem(ctx, sqlcdb.QualityProfileItem{
		ProfileID: int64(profileID), QualityID: int64(defID),
	})
	require.NoError(t, err)

	profile, err = store.GetQualityProfile(ctx, profileID)
	require.NoError(t, err)
	assert.Equal(t, expectedProfile, profile)
	rows, err = store.ListQualityProfileRows(ctx, profileID)
	require.NoError(t, err)
	assert.Equal(t, []sqlcdb.ListQualityProfileRowsRow{{
		QualityProfile:    sqlcdb.QualityProfile{ID: profileID, Name: "after", UpgradeAllowed: true, CutoffQualityID: sql.NullInt64{Int64: cutoff, Valid: true}},
		QualityDefinition: sqlcdb.QualityDefinition{ID: defID, QualityID: 1, Name: "q", PreferredSize: 100, MinSize: 10, MaxSize: 200, MediaType: "movie"},
	}}, rows)
}

func TestDeleteQualityProfileRemovesItems(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)

	baselineItems, err := store.ListQualityProfileItems(ctx)
	require.NoError(t, err)
	baselineProfiles, err := store.ListQualityProfiles(ctx)
	require.NoError(t, err)

	profileID, err := store.CreateQualityProfile(ctx, sqlcdb.QualityProfile{Name: "doomed"})
	require.NoError(t, err)
	defID, err := store.CreateQualityDefinition(ctx, sqlcdb.QualityDefinition{
		QualityID: 1, Name: "q", PreferredSize: 100, MinSize: 10, MaxSize: 200, MediaType: "movie",
	})
	require.NoError(t, err)
	_, err = store.CreateQualityProfileItem(ctx, sqlcdb.QualityProfileItem{
		ProfileID: int64(profileID), QualityID: int64(defID),
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

	profileID, err := store.CreateQualityProfile(ctx, sqlcdb.QualityProfile{Name: "batch"})
	require.NoError(t, err)
	def1ID, err := store.CreateQualityDefinition(ctx, sqlcdb.QualityDefinition{
		QualityID: 201, Name: "q1", PreferredSize: 100, MinSize: 10, MaxSize: 200, MediaType: "movie",
	})
	require.NoError(t, err)
	def2ID, err := store.CreateQualityDefinition(ctx, sqlcdb.QualityDefinition{
		QualityID: 202, Name: "q2", PreferredSize: 100, MinSize: 15, MaxSize: 200, MediaType: "movie",
	})
	require.NoError(t, err)

	// Empty batch is a no-op.
	err = store.CreateQualityProfileItems(ctx, nil)
	require.NoError(t, err)

	err = store.CreateQualityProfileItems(ctx, []sqlcdb.QualityProfileItem{
		{ProfileID: int64(profileID), QualityID: int64(def1ID)},
		{ProfileID: int64(profileID), QualityID: int64(def2ID)},
	})
	require.NoError(t, err)

	items, err := store.ListQualityProfileItems(ctx)
	require.NoError(t, err)
	require.Len(t, items, len(baseline)+2)

	var mine []sqlcdb.QualityProfileItem
	for _, item := range items {
		if item.ProfileID == int64(profileID) {
			mine = append(mine, item)
		}
	}
	require.Len(t, mine, 2)
	for _, item := range mine {
		assert.Positive(t, item.ID)
	}
	assert.ElementsMatch(t, []int64{int64(def1ID), int64(def2ID)},
		[]int64{mine[0].QualityID, mine[1].QualityID})

	err = store.DeleteQualityProfileItemsByProfileID(ctx, profileID)
	require.NoError(t, err)

	items, err = store.ListQualityProfileItems(ctx)
	require.NoError(t, err)
	assert.Len(t, items, len(baseline))
	for _, item := range items {
		assert.NotEqual(t, int64(profileID), item.ProfileID)
	}
}

func TestGetQualityProfileItemNotFound(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)

	_, err := store.GetQualityProfileItem(ctx, 1234)
	assert.ErrorIs(t, err, storage.ErrNotFound)
}
