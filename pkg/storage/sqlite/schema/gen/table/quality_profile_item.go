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

var QualityProfileItem = newQualityProfileItemTable("", "quality_profile_item", "")

type qualityProfileItemTable struct {
	sqlite.Table

	// Columns
	ID        sqlite.ColumnInteger
	ProfileID sqlite.ColumnInteger
	QualityID sqlite.ColumnInteger

	AllColumns     sqlite.ColumnList
	MutableColumns sqlite.ColumnList
}

type QualityProfileItemTable struct {
	qualityProfileItemTable

	EXCLUDED qualityProfileItemTable
}

// AS creates new QualityProfileItemTable with assigned alias
func (a QualityProfileItemTable) AS(alias string) *QualityProfileItemTable {
	return newQualityProfileItemTable(a.SchemaName(), a.TableName(), alias)
}

// Schema creates new QualityProfileItemTable with assigned schema name
func (a QualityProfileItemTable) FromSchema(schemaName string) *QualityProfileItemTable {
	return newQualityProfileItemTable(schemaName, a.TableName(), a.Alias())
}

// WithPrefix creates new QualityProfileItemTable with assigned table prefix
func (a QualityProfileItemTable) WithPrefix(prefix string) *QualityProfileItemTable {
	return newQualityProfileItemTable(a.SchemaName(), prefix+a.TableName(), a.TableName())
}

// WithSuffix creates new QualityProfileItemTable with assigned table suffix
func (a QualityProfileItemTable) WithSuffix(suffix string) *QualityProfileItemTable {
	return newQualityProfileItemTable(a.SchemaName(), a.TableName()+suffix, a.TableName())
}

func newQualityProfileItemTable(schemaName, tableName, alias string) *QualityProfileItemTable {
	return &QualityProfileItemTable{
		qualityProfileItemTable: newQualityProfileItemTableImpl(schemaName, tableName, alias),
		EXCLUDED:                newQualityProfileItemTableImpl("", "excluded", ""),
	}
}

func newQualityProfileItemTableImpl(schemaName, tableName, alias string) qualityProfileItemTable {
	var (
		IDColumn        = sqlite.IntegerColumn("id")
		ProfileIDColumn = sqlite.IntegerColumn("profile_id")
		QualityIDColumn = sqlite.IntegerColumn("quality_id")
		allColumns      = sqlite.ColumnList{IDColumn, ProfileIDColumn, QualityIDColumn}
		mutableColumns  = sqlite.ColumnList{ProfileIDColumn, QualityIDColumn}
	)

	return qualityProfileItemTable{
		Table: sqlite.NewTable(schemaName, tableName, alias, allColumns...),

		//Columns
		ID:        IDColumn,
		ProfileID: ProfileIDColumn,
		QualityID: QualityIDColumn,

		AllColumns:     allColumns,
		MutableColumns: mutableColumns,
	}
}