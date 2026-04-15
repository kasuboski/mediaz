package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/kasuboski/mediaz/pkg/storage"
	sqlite3 "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInit(t *testing.T) {
	store := initSqlite(t, context.Background())
	assert.NotNil(t, store)
}

// TestConcurrentWriteConflict proves why  is required alongside
// WAL mode and busy_timeout.
//
// In WAL mode, a deferred transaction (BEGIN) establishes a read snapshot on its
// first read. If another connection writes and commits before the first connection
// attempts its own write, SQLite returns SQLITE_BUSY_SNAPSHOT — an extended error
// code for which the busy-timeout handler is never invoked. The write fails
// immediately regardless of how large _busy_timeout is set.
//
// BEGIN IMMEDIATE acquires a write lock at transaction start. Contention is
// expressed as plain SQLITE_BUSY (which busy_timeout retries correctly) rather
// than SQLITE_BUSY_SNAPSHOT (which it cannot). Both concurrent writers succeed.
func TestConcurrentWriteConflict(t *testing.T) {
	ctx := context.Background()

	setupDB := func(t *testing.T, dsnSuffix string) *sql.DB {
		t.Helper()
		f, err := os.CreateTemp(t.TempDir(), "mediaz-txlock-*.db")
		require.NoError(t, err)
		f.Close()

		db, err := sql.Open("sqlite3", f.Name()+dsnSuffix)
		require.NoError(t, err)
		t.Cleanup(func() { db.Close() })

		_, err = db.ExecContext(ctx, "CREATE TABLE kv (key TEXT PRIMARY KEY, val TEXT)")
		require.NoError(t, err)
		_, err = db.ExecContext(ctx, "INSERT INTO kv VALUES ('k', 'initial')")
		require.NoError(t, err)
		return db
	}

	// Deferred transactions (the default) fail when a concurrent commit invalidates
	// the snapshot before the first connection tries to write.
	t.Run("deferred transactions get SQLITE_BUSY_SNAPSHOT after concurrent commit", func(t *testing.T) {
		db := setupDB(t, "?_journal_mode=WAL&_busy_timeout=5000")

		conn1, err := db.Conn(ctx)
		require.NoError(t, err)
		defer conn1.Close()

		conn2, err := db.Conn(ctx)
		require.NoError(t, err)
		defer conn2.Close()

		// conn1: begin deferred transaction and read, establishing a WAL snapshot.
		tx1, err := conn1.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer tx1.Rollback()

		var val string
		require.NoError(t, tx1.QueryRowContext(ctx, "SELECT val FROM kv WHERE key = 'k'").Scan(&val))

		// conn2: write and commit, advancing the WAL past conn1's snapshot.
		tx2, err := conn2.BeginTx(ctx, nil)
		require.NoError(t, err)
		_, err = tx2.ExecContext(ctx, "UPDATE kv SET val = 'conn2' WHERE key = 'k'")
		require.NoError(t, err)
		require.NoError(t, tx2.Commit())

		// conn1: tries to upgrade its stale read snapshot to a write transaction.
		// SQLite returns SQLITE_BUSY_SNAPSHOT; busy_timeout does not retry it.
		_, err = tx1.ExecContext(ctx, "UPDATE kv SET val = 'conn1' WHERE key = 'k'")
		require.Error(t, err, "expected write to fail with SQLITE_BUSY_SNAPSHOT")

		var sqliteErr sqlite3.Error
		require.True(t, errors.As(err, &sqliteErr), "expected a sqlite3.Error, got: %v", err)
		assert.Equal(t, sqlite3.ErrBusySnapshot, sqliteErr.ExtendedCode, "expected SQLITE_BUSY_SNAPSHOT (261)")
	})

	// Immediate transactions serialize correctly: BEGIN IMMEDIATE acquires the
	// write lock at transaction start, so contention surfaces as plain SQLITE_BUSY
	// (which busy_timeout retries) rather than SQLITE_BUSY_SNAPSHOT (which it cannot).
	//
	// This test replays the exact same read-then-competing-write scenario as the
	// deferred case above, but with _txlock=immediate in the DSN — both writers succeed.
	t.Run("immediate transactions serialize without snapshot conflict", func(t *testing.T) {
		// _txlock=immediate makes every BeginTx issue BEGIN IMMEDIATE, acquiring the
		// write lock up front. This is what the production DSN in New() now uses.
		db := setupDB(t, "?_journal_mode=WAL&_busy_timeout=5000&_txlock=immediate")

		conn1, err := db.Conn(ctx)
		require.NoError(t, err)
		defer conn1.Close()

		conn2, err := db.Conn(ctx)
		require.NoError(t, err)
		defer conn2.Close()

		// conn1: BEGIN IMMEDIATE — acquires the write lock at transaction start.
		tx1, err := conn1.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer tx1.Rollback()

		// conn1 reads, establishing the same pattern as the failing deferred test.
		// Unlike deferred, the snapshot cannot go stale: conn1 already holds the lock.
		var val string
		require.NoError(t, tx1.QueryRowContext(ctx, "SELECT val FROM kv WHERE key = 'k'").Scan(&val))

		// conn2: BEGIN IMMEDIATE in a goroutine. It blocks with SQLITE_BUSY because
		// conn1 holds the write lock. busy_timeout retries SQLITE_BUSY automatically,
		// so BeginTx will succeed once conn1 commits.
		conn2Err := make(chan error, 1)
		go func() {
			tx2, err := conn2.BeginTx(ctx, nil)
			if err != nil {
				conn2Err <- err
				return
			}
			_, err = tx2.ExecContext(ctx, "UPDATE kv SET val = 'conn2' WHERE key = 'k'")
			if err != nil {
				tx2.Rollback()
				conn2Err <- err
				return
			}
			conn2Err <- tx2.Commit()
		}()

		// Give conn2's goroutine a moment to reach BeginTx and start waiting.
		time.Sleep(50 * time.Millisecond)

		// conn1: write and commit. Releasing the write lock lets conn2 proceed.
		// Unlike the deferred case there is no SQLITE_BUSY_SNAPSHOT — conn2 was
		// blocked at BEGIN, not at write time.
		_, err = tx1.ExecContext(ctx, "UPDATE kv SET val = 'conn1' WHERE key = 'k'")
		require.NoError(t, err)
		require.NoError(t, tx1.Commit())

		// conn2 must succeed: it received retriable SQLITE_BUSY, not SQLITE_BUSY_SNAPSHOT.
		require.NoError(t, <-conn2Err)
	})
}

func ptr[A any](thing A) *A {
	return &thing
}

func initSqlite(t *testing.T, ctx context.Context) storage.Storage {
	store, err := New(ctx, ":memory:")
	assert.Nil(t, err)
	return store
}
