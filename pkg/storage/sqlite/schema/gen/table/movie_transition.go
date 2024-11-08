//
// Code generated by go-jet DO NOT EDIT.
//
// WARNING: Changes to this file may cause incorrect behavior
// and will be lost if the code is regenerated
//

package table

import (
	"github.com/go-jet/jet/v2/sqlite"
)

var MovieTransition = newMovieTransitionTable("", "movie_transition", "")

type movieTransitionTable struct {
	sqlite.Table

	// Columns
	ID         sqlite.ColumnInteger
	MovieID    sqlite.ColumnInteger
	ToState    sqlite.ColumnString
	FromState  sqlite.ColumnString
	MostRecent sqlite.ColumnBool
	SortKey    sqlite.ColumnInteger
	CreatedAt  sqlite.ColumnTimestamp
	UpdatedAt  sqlite.ColumnTimestamp

	AllColumns     sqlite.ColumnList
	MutableColumns sqlite.ColumnList
}

type MovieTransitionTable struct {
	movieTransitionTable

	EXCLUDED movieTransitionTable
}

// AS creates new MovieTransitionTable with assigned alias
func (a MovieTransitionTable) AS(alias string) *MovieTransitionTable {
	return newMovieTransitionTable(a.SchemaName(), a.TableName(), alias)
}

// Schema creates new MovieTransitionTable with assigned schema name
func (a MovieTransitionTable) FromSchema(schemaName string) *MovieTransitionTable {
	return newMovieTransitionTable(schemaName, a.TableName(), a.Alias())
}

// WithPrefix creates new MovieTransitionTable with assigned table prefix
func (a MovieTransitionTable) WithPrefix(prefix string) *MovieTransitionTable {
	return newMovieTransitionTable(a.SchemaName(), prefix+a.TableName(), a.TableName())
}

// WithSuffix creates new MovieTransitionTable with assigned table suffix
func (a MovieTransitionTable) WithSuffix(suffix string) *MovieTransitionTable {
	return newMovieTransitionTable(a.SchemaName(), a.TableName()+suffix, a.TableName())
}

func newMovieTransitionTable(schemaName, tableName, alias string) *MovieTransitionTable {
	return &MovieTransitionTable{
		movieTransitionTable: newMovieTransitionTableImpl(schemaName, tableName, alias),
		EXCLUDED:             newMovieTransitionTableImpl("", "excluded", ""),
	}
}

func newMovieTransitionTableImpl(schemaName, tableName, alias string) movieTransitionTable {
	var (
		IDColumn         = sqlite.IntegerColumn("id")
		MovieIDColumn    = sqlite.IntegerColumn("movie_id")
		ToStateColumn    = sqlite.StringColumn("to_state")
		FromStateColumn  = sqlite.StringColumn("from_state")
		MostRecentColumn = sqlite.BoolColumn("most_recent")
		SortKeyColumn    = sqlite.IntegerColumn("sort_key")
		CreatedAtColumn  = sqlite.TimestampColumn("created_at")
		UpdatedAtColumn  = sqlite.TimestampColumn("updated_at")
		allColumns       = sqlite.ColumnList{IDColumn, MovieIDColumn, ToStateColumn, FromStateColumn, MostRecentColumn, SortKeyColumn, CreatedAtColumn, UpdatedAtColumn}
		mutableColumns   = sqlite.ColumnList{MovieIDColumn, ToStateColumn, FromStateColumn, MostRecentColumn, SortKeyColumn, CreatedAtColumn, UpdatedAtColumn}
	)

	return movieTransitionTable{
		Table: sqlite.NewTable(schemaName, tableName, alias, allColumns...),

		//Columns
		ID:         IDColumn,
		MovieID:    MovieIDColumn,
		ToState:    ToStateColumn,
		FromState:  FromStateColumn,
		MostRecent: MostRecentColumn,
		SortKey:    SortKeyColumn,
		CreatedAt:  CreatedAtColumn,
		UpdatedAt:  UpdatedAtColumn,

		AllColumns:     allColumns,
		MutableColumns: mutableColumns,
	}
}