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

var QualityProfile = newQualityProfileTable("", "quality_profile", "")

type qualityProfileTable struct {
	sqlite.Table

	// Columns
	ID              sqlite.ColumnInteger
	Name            sqlite.ColumnString
	CutoffQualityID sqlite.ColumnInteger
	UpgradeAllowed  sqlite.ColumnBool

	AllColumns     sqlite.ColumnList
	MutableColumns sqlite.ColumnList
}

type QualityProfileTable struct {
	qualityProfileTable

	EXCLUDED qualityProfileTable
}

// AS creates new QualityProfileTable with assigned alias
func (a QualityProfileTable) AS(alias string) *QualityProfileTable {
	return newQualityProfileTable(a.SchemaName(), a.TableName(), alias)
}

// Schema creates new QualityProfileTable with assigned schema name
func (a QualityProfileTable) FromSchema(schemaName string) *QualityProfileTable {
	return newQualityProfileTable(schemaName, a.TableName(), a.Alias())
}

// WithPrefix creates new QualityProfileTable with assigned table prefix
func (a QualityProfileTable) WithPrefix(prefix string) *QualityProfileTable {
	return newQualityProfileTable(a.SchemaName(), prefix+a.TableName(), a.TableName())
}

// WithSuffix creates new QualityProfileTable with assigned table suffix
func (a QualityProfileTable) WithSuffix(suffix string) *QualityProfileTable {
	return newQualityProfileTable(a.SchemaName(), a.TableName()+suffix, a.TableName())
}

func newQualityProfileTable(schemaName, tableName, alias string) *QualityProfileTable {
	return &QualityProfileTable{
		qualityProfileTable: newQualityProfileTableImpl(schemaName, tableName, alias),
		EXCLUDED:            newQualityProfileTableImpl("", "excluded", ""),
	}
}

func newQualityProfileTableImpl(schemaName, tableName, alias string) qualityProfileTable {
	var (
		IDColumn              = sqlite.IntegerColumn("id")
		NameColumn            = sqlite.StringColumn("name")
		CutoffQualityIDColumn = sqlite.IntegerColumn("cutoff_quality_id")
		UpgradeAllowedColumn  = sqlite.BoolColumn("upgrade_allowed")
		allColumns            = sqlite.ColumnList{IDColumn, NameColumn, CutoffQualityIDColumn, UpgradeAllowedColumn}
		mutableColumns        = sqlite.ColumnList{NameColumn, CutoffQualityIDColumn, UpgradeAllowedColumn}
	)

	return qualityProfileTable{
		Table: sqlite.NewTable(schemaName, tableName, alias, allColumns...),

		//Columns
		ID:              IDColumn,
		Name:            NameColumn,
		CutoffQualityID: CutoffQualityIDColumn,
		UpgradeAllowed:  UpgradeAllowedColumn,

		AllColumns:     allColumns,
		MutableColumns: mutableColumns,
	}
}
