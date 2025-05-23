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

var Season = newSeasonTable("", "season", "")

type seasonTable struct {
	sqlite.Table

	// Columns
	ID               sqlite.ColumnInteger
	SeriesID         sqlite.ColumnInteger
	SeasonMetadataID sqlite.ColumnInteger
	Monitored        sqlite.ColumnInteger

	AllColumns     sqlite.ColumnList
	MutableColumns sqlite.ColumnList
}

type SeasonTable struct {
	seasonTable

	EXCLUDED seasonTable
}

// AS creates new SeasonTable with assigned alias
func (a SeasonTable) AS(alias string) *SeasonTable {
	return newSeasonTable(a.SchemaName(), a.TableName(), alias)
}

// Schema creates new SeasonTable with assigned schema name
func (a SeasonTable) FromSchema(schemaName string) *SeasonTable {
	return newSeasonTable(schemaName, a.TableName(), a.Alias())
}

// WithPrefix creates new SeasonTable with assigned table prefix
func (a SeasonTable) WithPrefix(prefix string) *SeasonTable {
	return newSeasonTable(a.SchemaName(), prefix+a.TableName(), a.TableName())
}

// WithSuffix creates new SeasonTable with assigned table suffix
func (a SeasonTable) WithSuffix(suffix string) *SeasonTable {
	return newSeasonTable(a.SchemaName(), a.TableName()+suffix, a.TableName())
}

func newSeasonTable(schemaName, tableName, alias string) *SeasonTable {
	return &SeasonTable{
		seasonTable: newSeasonTableImpl(schemaName, tableName, alias),
		EXCLUDED:    newSeasonTableImpl("", "excluded", ""),
	}
}

func newSeasonTableImpl(schemaName, tableName, alias string) seasonTable {
	var (
		IDColumn               = sqlite.IntegerColumn("id")
		SeriesIDColumn         = sqlite.IntegerColumn("series_id")
		SeasonMetadataIDColumn = sqlite.IntegerColumn("season_metadata_id")
		MonitoredColumn        = sqlite.IntegerColumn("monitored")
		allColumns             = sqlite.ColumnList{IDColumn, SeriesIDColumn, SeasonMetadataIDColumn, MonitoredColumn}
		mutableColumns         = sqlite.ColumnList{SeriesIDColumn, SeasonMetadataIDColumn, MonitoredColumn}
	)

	return seasonTable{
		Table: sqlite.NewTable(schemaName, tableName, alias, allColumns...),

		//Columns
		ID:               IDColumn,
		SeriesID:         SeriesIDColumn,
		SeasonMetadataID: SeasonMetadataIDColumn,
		Monitored:        MonitoredColumn,

		AllColumns:     allColumns,
		MutableColumns: mutableColumns,
	}
}
