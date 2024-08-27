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

var Metadata = newMetadataTable("", "Metadata", "")

type metadataTable struct {
	sqlite.Table

	// Columns
	ID             sqlite.ColumnInteger
	Enable         sqlite.ColumnInteger
	Name           sqlite.ColumnString
	Implementation sqlite.ColumnString
	Settings       sqlite.ColumnString
	ConfigContract sqlite.ColumnString

	AllColumns     sqlite.ColumnList
	MutableColumns sqlite.ColumnList
}

type MetadataTable struct {
	metadataTable

	EXCLUDED metadataTable
}

// AS creates new MetadataTable with assigned alias
func (a MetadataTable) AS(alias string) *MetadataTable {
	return newMetadataTable(a.SchemaName(), a.TableName(), alias)
}

// Schema creates new MetadataTable with assigned schema name
func (a MetadataTable) FromSchema(schemaName string) *MetadataTable {
	return newMetadataTable(schemaName, a.TableName(), a.Alias())
}

// WithPrefix creates new MetadataTable with assigned table prefix
func (a MetadataTable) WithPrefix(prefix string) *MetadataTable {
	return newMetadataTable(a.SchemaName(), prefix+a.TableName(), a.TableName())
}

// WithSuffix creates new MetadataTable with assigned table suffix
func (a MetadataTable) WithSuffix(suffix string) *MetadataTable {
	return newMetadataTable(a.SchemaName(), a.TableName()+suffix, a.TableName())
}

func newMetadataTable(schemaName, tableName, alias string) *MetadataTable {
	return &MetadataTable{
		metadataTable: newMetadataTableImpl(schemaName, tableName, alias),
		EXCLUDED:      newMetadataTableImpl("", "excluded", ""),
	}
}

func newMetadataTableImpl(schemaName, tableName, alias string) metadataTable {
	var (
		IDColumn             = sqlite.IntegerColumn("Id")
		EnableColumn         = sqlite.IntegerColumn("Enable")
		NameColumn           = sqlite.StringColumn("Name")
		ImplementationColumn = sqlite.StringColumn("Implementation")
		SettingsColumn       = sqlite.StringColumn("Settings")
		ConfigContractColumn = sqlite.StringColumn("ConfigContract")
		allColumns           = sqlite.ColumnList{IDColumn, EnableColumn, NameColumn, ImplementationColumn, SettingsColumn, ConfigContractColumn}
		mutableColumns       = sqlite.ColumnList{EnableColumn, NameColumn, ImplementationColumn, SettingsColumn, ConfigContractColumn}
	)

	return metadataTable{
		Table: sqlite.NewTable(schemaName, tableName, alias, allColumns...),

		//Columns
		ID:             IDColumn,
		Enable:         EnableColumn,
		Name:           NameColumn,
		Implementation: ImplementationColumn,
		Settings:       SettingsColumn,
		ConfigContract: ConfigContractColumn,

		AllColumns:     allColumns,
		MutableColumns: mutableColumns,
	}
}
