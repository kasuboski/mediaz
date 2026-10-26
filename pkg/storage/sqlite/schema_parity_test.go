package sqlite

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// sqlc checks SQL against the snapshot, while production starts from migrations.
// Compare their structural metadata so a stale snapshot cannot silently produce
// queries/models for a different database (for example, missing indexer sources).
func TestSQLCSchemaMatchesMigrations(t *testing.T) {
	ctx := context.Background()
	store, err := New(ctx, ":memory:")
	require.NoError(t, err)
	migrated := store.(*SQLite).db
	t.Cleanup(func() { require.NoError(t, migrated.Close()) })

	snapshot, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, snapshot.Close()) })
	schema, err := os.ReadFile("schema/schema.sql")
	require.NoError(t, err)
	_, err = snapshot.ExecContext(ctx, string(schema))
	require.NoError(t, err)

	const tables = `SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' AND name != 'schema_migrations' ORDER BY name`
	actualTables := schemaMetadata(t, migrated, tables)
	require.Equal(t, actualTables, schemaMetadata(t, snapshot, tables))
	for _, row := range actualTables {
		name := row[0].(string)
		t.Run(name, func(t *testing.T) {
			for _, query := range []string{
				`SELECT cid, name, type, "notnull", dflt_value, pk FROM pragma_table_info(?) ORDER BY cid`,
				`SELECT "table", "from", "to", on_update, on_delete, match FROM pragma_foreign_key_list(?) ORDER BY "table", "from", "to"`,
				`SELECT name, "unique", origin, partial FROM pragma_index_list(?) ORDER BY name`,
			} {
				require.Equal(t, schemaMetadata(t, migrated, query, name), schemaMetadata(t, snapshot, query, name), query)
			}
			for _, index := range schemaMetadata(t, migrated, `SELECT name FROM pragma_index_list(?) ORDER BY name`, name) {
				const query = `SELECT seqno, cid, name, "desc", coll, "key" FROM pragma_index_xinfo(?) ORDER BY seqno`
				require.Equal(t, schemaMetadata(t, migrated, query, index[0]), schemaMetadata(t, snapshot, query, index[0]), index[0])
			}
		})
	}
}

func schemaMetadata(t *testing.T, db *sql.DB, query string, args ...any) [][]any {
	t.Helper()
	rows, err := db.Query(query, args...)
	require.NoError(t, err)
	defer rows.Close()
	columns, err := rows.Columns()
	require.NoError(t, err)
	result := make([][]any, 0)
	for rows.Next() {
		values := make([]any, len(columns))
		targets := make([]any, len(columns))
		for i := range values {
			targets[i] = &values[i]
		}
		require.NoError(t, rows.Scan(targets...))
		result = append(result, values)
	}
	require.NoError(t, rows.Err())
	return result
}
