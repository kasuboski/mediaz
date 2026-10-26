package sqlite

import (
	"database/sql"
	"testing"

	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQualityProfileCutoffReferencesDefinitionID(t *testing.T) {
	ctx := t.Context()
	store := initSqlite(t, ctx)
	before, err := store.ListQualityProfiles(ctx)
	require.NoError(t, err)
	// The domain quality enum is deliberately not a definition primary key.
	const qualityEnum = int64(987654321)
	def := sqlcdb.QualityDefinition{QualityID: qualityEnum, Name: "cutoff-only", MediaType: "movie", MinSize: 1, PreferredSize: 2, MaxSize: 3}
	defID, err := store.CreateQualityDefinition(ctx, def)
	require.NoError(t, err)
	def.ID = defID
	require.NotEqual(t, qualityEnum, defID)
	missing := sql.NullInt64{Int64: qualityEnum, Valid: true}
	_, err = store.CreateQualityProfile(ctx, sqlcdb.QualityProfile{Name: "invalid", CutoffQualityID: missing, UpgradeAllowed: true})
	requireConstraint(t, err, sqlite3.ErrConstraintForeignKey)
	after, err := store.ListQualityProfiles(ctx)
	require.NoError(t, err)
	assert.Equal(t, before, after)

	profile := sqlcdb.QualityProfile{Name: "cutoff-only", CutoffQualityID: sql.NullInt64{Int64: defID, Valid: true}, UpgradeAllowed: true}
	profile.ID, err = store.CreateQualityProfile(ctx, profile)
	require.NoError(t, err)
	// No profile item references this definition: the cutoff FK must protect it alone.
	items, err := store.ListQualityProfileRows(ctx, profile.ID)
	require.NoError(t, err)
	require.Empty(t, items)
	requireConstraint(t, store.DeleteQualityDefinition(ctx, defID), sqlite3.ErrConstraintTrigger)
	gotDef, err := store.GetQualityDefinition(ctx, defID)
	require.NoError(t, err)
	assert.Equal(t, def, gotDef)

	invalid := profile
	invalid.Name = "must not persist"
	invalid.CutoffQualityID = missing
	requireConstraint(t, store.UpdateQualityProfile(ctx, profile.ID, invalid), sqlite3.ErrConstraintForeignKey)
	got, err := store.GetQualityProfile(ctx, profile.ID)
	require.NoError(t, err)
	assert.Equal(t, profile, got)

	// Clearing an optional cutoff permits definition deletion without deleting the profile.
	profile.CutoffQualityID = sql.NullInt64{}
	require.NoError(t, store.UpdateQualityProfile(ctx, profile.ID, profile))
	require.NoError(t, store.DeleteQualityDefinition(ctx, defID))
	got, err = store.GetQualityProfile(ctx, profile.ID)
	require.NoError(t, err)
	assert.Equal(t, profile, got)
	nullProfile := sqlcdb.QualityProfile{Name: "null cutoff"}
	nullProfile.ID, err = store.CreateQualityProfile(ctx, nullProfile)
	require.NoError(t, err)
	got, err = store.GetQualityProfile(ctx, nullProfile.ID)
	require.NoError(t, err)
	assert.Equal(t, nullProfile, got)
}
