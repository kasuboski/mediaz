package sqlite

import (
	"context"
	"testing"

	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewStorage(t *testing.T) {
	store := initSqlite(t, context.Background())
	assert.NotNil(t, store)
}

func initSqlite(t *testing.T, ctx context.Context) storage.Storage {
	store, err := New(ctx, ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.(*SQLite).db.Close()) })
	return store
}
