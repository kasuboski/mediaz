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

var Config = newConfigTable("", "Config", "")

type configTable struct {
	sqlite.Table

	// Columns
	ID    sqlite.ColumnInteger
	Key   sqlite.ColumnString
	Value sqlite.ColumnString

	AllColumns     sqlite.ColumnList
	MutableColumns sqlite.ColumnList
}

type ConfigTable struct {
	configTable

	EXCLUDED configTable
}

// AS creates new ConfigTable with assigned alias
func (a ConfigTable) AS(alias string) *ConfigTable {
	return newConfigTable(a.SchemaName(), a.TableName(), alias)
}

// Schema creates new ConfigTable with assigned schema name
func (a ConfigTable) FromSchema(schemaName string) *ConfigTable {
	return newConfigTable(schemaName, a.TableName(), a.Alias())
}

// WithPrefix creates new ConfigTable with assigned table prefix
func (a ConfigTable) WithPrefix(prefix string) *ConfigTable {
	return newConfigTable(a.SchemaName(), prefix+a.TableName(), a.TableName())
}

// WithSuffix creates new ConfigTable with assigned table suffix
func (a ConfigTable) WithSuffix(suffix string) *ConfigTable {
	return newConfigTable(a.SchemaName(), a.TableName()+suffix, a.TableName())
}

func newConfigTable(schemaName, tableName, alias string) *ConfigTable {
	return &ConfigTable{
		configTable: newConfigTableImpl(schemaName, tableName, alias),
		EXCLUDED:    newConfigTableImpl("", "excluded", ""),
	}
}

func newConfigTableImpl(schemaName, tableName, alias string) configTable {
	var (
		IDColumn       = sqlite.IntegerColumn("Id")
		KeyColumn      = sqlite.StringColumn("Key")
		ValueColumn    = sqlite.StringColumn("Value")
		allColumns     = sqlite.ColumnList{IDColumn, KeyColumn, ValueColumn}
		mutableColumns = sqlite.ColumnList{KeyColumn, ValueColumn}
	)

	return configTable{
		Table: sqlite.NewTable(schemaName, tableName, alias, allColumns...),

		//Columns
		ID:    IDColumn,
		Key:   KeyColumn,
		Value: ValueColumn,

		AllColumns:     allColumns,
		MutableColumns: mutableColumns,
	}
}
