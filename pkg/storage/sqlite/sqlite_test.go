package sqlite

import (
	"context"
	"testing"

	"github.com/kasuboski/mediaz/pkg/storage"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInit(t *testing.T) {
	store := initSqlite(t, context.Background())
	assert.NotNil(t, store)
}

// TestConnectionPragmas verifies that per-connection PRAGMAs (busy_timeout,
// synchronous) are applied to every connection the pool opens, not just the
// first one used during New().
func TestConnectionPragmas(t *testing.T) {
	ctx := context.Background()
	store, err := New(ctx, ":memory:")
	require.NoError(t, err)

	s := store.(*SQLite)

	// Explicitly acquire a connection so the pool opens (or reuses) one,
	// triggering the DSN hooks.
	conn, err := s.db.Conn(ctx)
	require.NoError(t, err)
	defer conn.Close()

	var busyTimeout int
	err = conn.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&busyTimeout)
	require.NoError(t, err)
	assert.Equal(t, 5000, busyTimeout, "busy_timeout should be 5000ms on every pool connection")

	var synchronous string
	err = conn.QueryRowContext(ctx, "PRAGMA synchronous").Scan(&synchronous)
	require.NoError(t, err)
	// SQLite returns the numeric value: 0=OFF, 1=NORMAL, 2=FULL, 3=EXTRA
	assert.Equal(t, "1", synchronous, "synchronous should be NORMAL (1) on every pool connection")
}

func ptr[A any](thing A) *A {
	return &thing
}

func initSqlite(t *testing.T, ctx context.Context) storage.Storage {
	store, err := New(ctx, ":memory:")
	assert.Nil(t, err)
	return store
}
