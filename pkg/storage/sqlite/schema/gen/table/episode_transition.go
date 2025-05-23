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

var EpisodeTransition = newEpisodeTransitionTable("", "episode_transition", "")

type episodeTransitionTable struct {
	sqlite.Table

	// Columns
	ID                     sqlite.ColumnInteger
	EpisodeID              sqlite.ColumnInteger
	ToState                sqlite.ColumnString
	FromState              sqlite.ColumnString
	MostRecent             sqlite.ColumnBool
	SortKey                sqlite.ColumnInteger
	DownloadClientID       sqlite.ColumnInteger
	DownloadID             sqlite.ColumnString
	IsEntireSeasonDownload sqlite.ColumnBool
	CreatedAt              sqlite.ColumnTimestamp
	UpdatedAt              sqlite.ColumnTimestamp

	AllColumns     sqlite.ColumnList
	MutableColumns sqlite.ColumnList
}

type EpisodeTransitionTable struct {
	episodeTransitionTable

	EXCLUDED episodeTransitionTable
}

// AS creates new EpisodeTransitionTable with assigned alias
func (a EpisodeTransitionTable) AS(alias string) *EpisodeTransitionTable {
	return newEpisodeTransitionTable(a.SchemaName(), a.TableName(), alias)
}

// Schema creates new EpisodeTransitionTable with assigned schema name
func (a EpisodeTransitionTable) FromSchema(schemaName string) *EpisodeTransitionTable {
	return newEpisodeTransitionTable(schemaName, a.TableName(), a.Alias())
}

// WithPrefix creates new EpisodeTransitionTable with assigned table prefix
func (a EpisodeTransitionTable) WithPrefix(prefix string) *EpisodeTransitionTable {
	return newEpisodeTransitionTable(a.SchemaName(), prefix+a.TableName(), a.TableName())
}

// WithSuffix creates new EpisodeTransitionTable with assigned table suffix
func (a EpisodeTransitionTable) WithSuffix(suffix string) *EpisodeTransitionTable {
	return newEpisodeTransitionTable(a.SchemaName(), a.TableName()+suffix, a.TableName())
}

func newEpisodeTransitionTable(schemaName, tableName, alias string) *EpisodeTransitionTable {
	return &EpisodeTransitionTable{
		episodeTransitionTable: newEpisodeTransitionTableImpl(schemaName, tableName, alias),
		EXCLUDED:               newEpisodeTransitionTableImpl("", "excluded", ""),
	}
}

func newEpisodeTransitionTableImpl(schemaName, tableName, alias string) episodeTransitionTable {
	var (
		IDColumn                     = sqlite.IntegerColumn("id")
		EpisodeIDColumn              = sqlite.IntegerColumn("episode_id")
		ToStateColumn                = sqlite.StringColumn("to_state")
		FromStateColumn              = sqlite.StringColumn("from_state")
		MostRecentColumn             = sqlite.BoolColumn("most_recent")
		SortKeyColumn                = sqlite.IntegerColumn("sort_key")
		DownloadClientIDColumn       = sqlite.IntegerColumn("download_client_id")
		DownloadIDColumn             = sqlite.StringColumn("download_id")
		IsEntireSeasonDownloadColumn = sqlite.BoolColumn("is_entire_season_download")
		CreatedAtColumn              = sqlite.TimestampColumn("created_at")
		UpdatedAtColumn              = sqlite.TimestampColumn("updated_at")
		allColumns                   = sqlite.ColumnList{IDColumn, EpisodeIDColumn, ToStateColumn, FromStateColumn, MostRecentColumn, SortKeyColumn, DownloadClientIDColumn, DownloadIDColumn, IsEntireSeasonDownloadColumn, CreatedAtColumn, UpdatedAtColumn}
		mutableColumns               = sqlite.ColumnList{EpisodeIDColumn, ToStateColumn, FromStateColumn, MostRecentColumn, SortKeyColumn, DownloadClientIDColumn, DownloadIDColumn, IsEntireSeasonDownloadColumn, CreatedAtColumn, UpdatedAtColumn}
	)

	return episodeTransitionTable{
		Table: sqlite.NewTable(schemaName, tableName, alias, allColumns...),

		//Columns
		ID:                     IDColumn,
		EpisodeID:              EpisodeIDColumn,
		ToState:                ToStateColumn,
		FromState:              FromStateColumn,
		MostRecent:             MostRecentColumn,
		SortKey:                SortKeyColumn,
		DownloadClientID:       DownloadClientIDColumn,
		DownloadID:             DownloadIDColumn,
		IsEntireSeasonDownload: IsEntireSeasonDownloadColumn,
		CreatedAt:              CreatedAtColumn,
		UpdatedAt:              UpdatedAtColumn,

		AllColumns:     allColumns,
		MutableColumns: mutableColumns,
	}
}
