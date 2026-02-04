package sqlite

import (
	"context"
	"testing"

	"github.com/kasuboski/mediaz/pkg/storage"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
)

func TestInit(t *testing.T) {
	store := initSqlite(t, context.Background())
	assert.NotNil(t, store)
}

func ptr[A any](thing A) *A {
	return &thing
}

func initSqlite(t *testing.T, ctx context.Context) storage.Storage {
	store, err := New(ctx, ":memory:")
	assert.Nil(t, err)
	return store
}
