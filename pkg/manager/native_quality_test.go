package manager

import (
	"context"
	"database/sql"
	"testing"

	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQualityServiceNativeProfileRows(t *testing.T) {
	ctx := context.Background()
	store := newQualityServiceStore(t)
	service := NewQualityService(store)
	cutoff := int64(7)
	profile, err := service.AddQualityProfile(ctx, AddQualityProfileRequest{
		Name: "Native profile", CutoffQualityID: &cutoff, UpgradeAllowed: true, QualityIDs: []int64{3, 7},
	})
	require.NoError(t, err)
	expected := sqlcdb.QualityProfile{ID: profile.ID, Name: "Native profile", CutoffQualityID: sql.NullInt64{Int64: 7, Valid: true}, UpgradeAllowed: true}
	assert.Equal(t, expected, profile)
	rows, err := service.ListQualityProfileRows(ctx, profile.ID)
	require.NoError(t, err)
	first, err := service.GetQualityDefinition(ctx, 3)
	require.NoError(t, err)
	second, err := service.GetQualityDefinition(ctx, 7)
	require.NoError(t, err)
	assert.Equal(t, []sqlcdb.ListQualityProfileRowsRow{
		{QualityProfile: expected, QualityDefinition: first},
		{QualityProfile: expected, QualityDefinition: second},
	}, rows)

	updated, err := service.UpdateQualityProfile(ctx, profile.ID, UpdateQualityProfileRequest{Name: "No cutoff", QualityIDs: []int64{3}})
	require.NoError(t, err)
	expected = sqlcdb.QualityProfile{ID: profile.ID, Name: "No cutoff"}
	assert.Equal(t, expected, updated)
	rows, err = service.ListQualityProfileRows(ctx, profile.ID)
	require.NoError(t, err)
	assert.Equal(t, []sqlcdb.ListQualityProfileRowsRow{{QualityProfile: expected, QualityDefinition: first}}, rows)
}

func TestQualityServiceNativeEmptyProfile(t *testing.T) {
	ctx := context.Background()
	store := newQualityServiceStore(t)
	service := NewQualityService(store)
	id, err := store.CreateQualityProfile(ctx, sqlcdb.QualityProfile{Name: "No items"})
	require.NoError(t, err)
	profile, err := service.GetQualityProfile(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, sqlcdb.QualityProfile{ID: id, Name: "No items"}, profile)
	rows, err := service.ListQualityProfileRows(ctx, id)
	require.NoError(t, err)
	assert.Empty(t, rows)
}
