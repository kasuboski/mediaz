package sqlite

import (
	"context"
	"testing"

	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQualityDefinitionStorage(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)

	// Migrations seed the default quality definitions; capture the baseline.
	baseline, err := store.ListQualityDefinitions(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, baseline)

	definition := model.QualityDefinition{
		QualityID:     4,
		Name:          "Bluray-720p-test",
		PreferredSize: 1999,
		MinSize:       25.7,
		MaxSize:       2000,
		MediaType:     "movie",
	}

	id, err := store.CreateQualityDefinition(ctx, definition)
	require.NoError(t, err)
	assert.NotZero(t, id)

	stored, err := store.GetQualityDefinition(ctx, id)
	require.NoError(t, err)
	definition.ID = int32(id)
	assert.Equal(t, definition, stored)

	definitions, err := store.ListQualityDefinitions(ctx)
	require.NoError(t, err)
	require.Len(t, definitions, len(baseline)+1)
	assert.Contains(t, definitions, &definition)

	// Updates change the editable columns but never quality_id.
	definition.Name = "Bluray-1080p-test"
	definition.MediaType = "episode"
	definition.MinSize = 30
	definition.MaxSize = 3000
	definition.PreferredSize = 2000
	err = store.UpdateQualityDefinition(ctx, id, definition)
	require.NoError(t, err)

	stored, err = store.GetQualityDefinition(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, definition, stored)

	err = store.DeleteQualityDefinition(ctx, id)
	require.NoError(t, err)

	definitions, err = store.ListQualityDefinitions(ctx)
	require.NoError(t, err)
	assert.Len(t, definitions, len(baseline))
}

func TestGetQualityDefinitionNotFound(t *testing.T) {
	ctx := context.Background()
	store := initSqlite(t, ctx)

	_, err := store.GetQualityDefinition(ctx, 1234)
	assert.ErrorIs(t, err, storage.ErrNotFound)
}
