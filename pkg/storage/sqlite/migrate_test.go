package sqlite

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigration_000001_FreshDatabase(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "test.db")
	ctx := context.Background()

	store, err := New(t.Context(), tmpFile)
	require.NoError(t, err)

	err = store.RunMigrations(ctx)
	require.NoError(t, err)

	profiles, err := store.ListQualityProfiles(ctx)
	require.NoError(t, err)
	assert.Equal(t, 6, len(profiles))

	sqliteStore := store.(*SQLite)
	version, dirty, err := sqliteStore.GetMigrationVersion()
	require.NoError(t, err)
	assert.Equal(t, uint(8), version)
	assert.False(t, dirty)

	profile1, err := store.GetQualityProfile(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, "Standard Definition", profile1.Name)
}

func TestMigration_000001_LegacyDatabase(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "legacy.db")
	ctx := context.Background()

	legacyStore, err := New(t.Context(), tmpFile)
	require.NoError(t, err)

	schemas, err := storage.GetSchemas()
	require.NoError(t, err)
	err = legacyStore.Init(ctx, schemas...)
	require.NoError(t, err)

	store, err := New(t.Context(), tmpFile)
	require.NoError(t, err)

	err = store.RunMigrations(ctx)
	require.NoError(t, err)

	profiles, err := store.ListQualityProfiles(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(profiles), 6)

	sqliteStore := store.(*SQLite)
	version, dirty, err := sqliteStore.GetMigrationVersion()
	require.NoError(t, err)
	assert.Equal(t, uint(8), version)
	assert.False(t, dirty)
}

func TestMigration_000002_UnmodifiedDefaults(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "test.db")
	ctx := context.Background()

	createV1DatabaseWithDefaults(t, tmpFile)

	store, err := New(t.Context(), tmpFile)
	require.NoError(t, err)

	err = store.RunMigrations(ctx)
	require.NoError(t, err)

	for _, profileID := range []int64{1, 2, 3, 4, 5, 6} {
		profile, err := store.GetQualityProfile(ctx, profileID)
		require.NoError(t, err)
		assert.Nil(t, profile.CutoffQualityID, "Profile %d should have NULL cutoff", profileID)
		assert.False(t, profile.UpgradeAllowed, "Profile %d should have upgrade_allowed=FALSE", profileID)
	}
}

func TestMigration_000002_PreservesModifiedProfile(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "test.db")
	ctx := context.Background()

	createV1DatabaseWithDefaults(t, tmpFile)

	tempStore, err := New(t.Context(), tmpFile)
	require.NoError(t, err)

	err = tempStore.RunMigrations(ctx)
	require.NoError(t, err)

	cutoffID := int32(2)
	err = tempStore.UpdateQualityProfile(ctx, 1, model.QualityProfile{
		Name:            "Custom SD",
		CutoffQualityID: &cutoffID,
		UpgradeAllowed:  true,
	})
	require.NoError(t, err)

	store, err := New(t.Context(), tmpFile)
	require.NoError(t, err)

	err = store.RunMigrations(ctx)
	require.NoError(t, err)

	profile1, err := store.GetQualityProfile(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, "Custom SD", profile1.Name)
	assert.NotNil(t, profile1.CutoffQualityID)
	assert.Equal(t, int32(2), *profile1.CutoffQualityID)
	assert.True(t, profile1.UpgradeAllowed)

	for _, profileID := range []int64{2, 3, 4, 5, 6} {
		profile, err := store.GetQualityProfile(ctx, profileID)
		require.NoError(t, err)
		assert.Nil(t, profile.CutoffQualityID, "Profile %d should have NULL cutoff", profileID)
		assert.False(t, profile.UpgradeAllowed, "Profile %d should have upgrade_allowed=FALSE", profileID)
	}
}

func TestMigration_000002_PreservesMultipleModifications(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "test.db")
	ctx := context.Background()

	createV1DatabaseWithDefaults(t, tmpFile)

	tempStore, err := New(t.Context(), tmpFile)
	require.NoError(t, err)

	err = tempStore.RunMigrations(ctx)
	require.NoError(t, err)

	cutoffID4 := int32(4)
	err = tempStore.UpdateQualityProfile(ctx, 2, model.QualityProfile{
		Name:            "High Definition",
		CutoffQualityID: &cutoffID4,
		UpgradeAllowed:  true,
	})
	require.NoError(t, err)

	cutoffID13 := int32(13)
	err = tempStore.UpdateQualityProfile(ctx, 3, model.QualityProfile{
		Name:            "Ultra High Definition",
		CutoffQualityID: &cutoffID13,
		UpgradeAllowed:  false,
	})
	require.NoError(t, err)

	store, err := New(t.Context(), tmpFile)
	require.NoError(t, err)

	err = store.RunMigrations(ctx)
	require.NoError(t, err)

	profile2, err := store.GetQualityProfile(ctx, 2)
	require.NoError(t, err)
	assert.NotNil(t, profile2.CutoffQualityID)
	assert.Equal(t, int32(4), *profile2.CutoffQualityID)
	assert.True(t, profile2.UpgradeAllowed)

	profile3, err := store.GetQualityProfile(ctx, 3)
	require.NoError(t, err)
	assert.NotNil(t, profile3.CutoffQualityID)
	assert.Equal(t, int32(13), *profile3.CutoffQualityID)
	assert.False(t, profile3.UpgradeAllowed)

	for _, profileID := range []int64{1, 4, 5, 6} {
		profile, err := store.GetQualityProfile(ctx, profileID)
		require.NoError(t, err)
		assert.Nil(t, profile.CutoffQualityID, "Profile %d should have NULL cutoff", profileID)
		assert.False(t, profile.UpgradeAllowed, "Profile %d should have upgrade_allowed=FALSE", profileID)
	}
}

func TestMigration_000002_FixesForeignKey(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "test.db")

	createV1DatabaseWithDefaults(t, tmpFile)

	store, err := New(t.Context(), tmpFile)
	require.NoError(t, err)

	err = store.RunMigrations(context.Background())
	require.NoError(t, err)

	sqliteStore := store.(*SQLite)
	rows, err := sqliteStore.db.Query(`
		SELECT sql FROM sqlite_master
		WHERE type='table' AND name='quality_profile_item'
	`)
	require.NoError(t, err)
	defer rows.Close()

	var schema string
	require.True(t, rows.Next())
	err = rows.Scan(&schema)
	require.NoError(t, err)

	assert.Contains(t, schema, `REFERENCES "quality_definition" ("id")`)
	assert.NotContains(t, schema, `REFERENCES "quality_definition" ("quality_id")`)
}

func createV1DatabaseWithDefaults(t *testing.T, dbPath string) {
	migration001Up, err := os.ReadFile("migrations/000001_initial_schema.up.sql")
	require.NoError(t, err)

	db, err := sql.Open("sqlite3", dbPath)
	require.NoError(t, err)
	defer db.Close()

	_, err = db.Exec(string(migration001Up))
	require.NoError(t, err)
}
