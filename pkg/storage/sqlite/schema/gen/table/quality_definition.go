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

var QualityDefinition = newQualityDefinitionTable("", "quality_definition", "")

type qualityDefinitionTable struct {
	sqlite.Table

	// Columns
	ID            sqlite.ColumnInteger
	QualityID     sqlite.ColumnInteger
	Name          sqlite.ColumnString
	PreferredSize sqlite.ColumnFloat
	MinSize       sqlite.ColumnFloat
	MaxSize       sqlite.ColumnFloat
	MediaType     sqlite.ColumnString

	AllColumns     sqlite.ColumnList
	MutableColumns sqlite.ColumnList
}

type QualityDefinitionTable struct {
	qualityDefinitionTable

	EXCLUDED qualityDefinitionTable
}

// AS creates new QualityDefinitionTable with assigned alias
func (a QualityDefinitionTable) AS(alias string) *QualityDefinitionTable {
	return newQualityDefinitionTable(a.SchemaName(), a.TableName(), alias)
}

// Schema creates new QualityDefinitionTable with assigned schema name
func (a QualityDefinitionTable) FromSchema(schemaName string) *QualityDefinitionTable {
	return newQualityDefinitionTable(schemaName, a.TableName(), a.Alias())
}

// WithPrefix creates new QualityDefinitionTable with assigned table prefix
func (a QualityDefinitionTable) WithPrefix(prefix string) *QualityDefinitionTable {
	return newQualityDefinitionTable(a.SchemaName(), prefix+a.TableName(), a.TableName())
}

// WithSuffix creates new QualityDefinitionTable with assigned table suffix
func (a QualityDefinitionTable) WithSuffix(suffix string) *QualityDefinitionTable {
	return newQualityDefinitionTable(a.SchemaName(), a.TableName()+suffix, a.TableName())
}

func newQualityDefinitionTable(schemaName, tableName, alias string) *QualityDefinitionTable {
	return &QualityDefinitionTable{
		qualityDefinitionTable: newQualityDefinitionTableImpl(schemaName, tableName, alias),
		EXCLUDED:               newQualityDefinitionTableImpl("", "excluded", ""),
	}
}

func newQualityDefinitionTableImpl(schemaName, tableName, alias string) qualityDefinitionTable {
	var (
		IDColumn            = sqlite.IntegerColumn("id")
		QualityIDColumn     = sqlite.IntegerColumn("quality_id")
		NameColumn          = sqlite.StringColumn("name")
		PreferredSizeColumn = sqlite.FloatColumn("preferred_size")
		MinSizeColumn       = sqlite.FloatColumn("min_size")
		MaxSizeColumn       = sqlite.FloatColumn("max_size")
		MediaTypeColumn     = sqlite.StringColumn("media_type")
		allColumns          = sqlite.ColumnList{IDColumn, QualityIDColumn, NameColumn, PreferredSizeColumn, MinSizeColumn, MaxSizeColumn, MediaTypeColumn}
		mutableColumns      = sqlite.ColumnList{QualityIDColumn, NameColumn, PreferredSizeColumn, MinSizeColumn, MaxSizeColumn, MediaTypeColumn}
	)

	return qualityDefinitionTable{
		Table: sqlite.NewTable(schemaName, tableName, alias, allColumns...),

		//Columns
		ID:            IDColumn,
		QualityID:     QualityIDColumn,
		Name:          NameColumn,
		PreferredSize: PreferredSizeColumn,
		MinSize:       MinSizeColumn,
		MaxSize:       MaxSizeColumn,
		MediaType:     MediaTypeColumn,

		AllColumns:     allColumns,
		MutableColumns: mutableColumns,
	}
}
