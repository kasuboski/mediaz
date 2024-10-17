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

var DownloadClient = newDownloadClientTable("", "download_client", "")

type downloadClientTable struct {
	sqlite.Table

	// Columns
	ID             sqlite.ColumnInteger
	Type           sqlite.ColumnString
	Implementation sqlite.ColumnString
	Scheme         sqlite.ColumnString
	Host           sqlite.ColumnString
	Port           sqlite.ColumnInteger
	Directory      sqlite.ColumnString

	AllColumns     sqlite.ColumnList
	MutableColumns sqlite.ColumnList
}

type DownloadClientTable struct {
	downloadClientTable

	EXCLUDED downloadClientTable
}

// AS creates new DownloadClientTable with assigned alias
func (a DownloadClientTable) AS(alias string) *DownloadClientTable {
	return newDownloadClientTable(a.SchemaName(), a.TableName(), alias)
}

// Schema creates new DownloadClientTable with assigned schema name
func (a DownloadClientTable) FromSchema(schemaName string) *DownloadClientTable {
	return newDownloadClientTable(schemaName, a.TableName(), a.Alias())
}

// WithPrefix creates new DownloadClientTable with assigned table prefix
func (a DownloadClientTable) WithPrefix(prefix string) *DownloadClientTable {
	return newDownloadClientTable(a.SchemaName(), prefix+a.TableName(), a.TableName())
}

// WithSuffix creates new DownloadClientTable with assigned table suffix
func (a DownloadClientTable) WithSuffix(suffix string) *DownloadClientTable {
	return newDownloadClientTable(a.SchemaName(), a.TableName()+suffix, a.TableName())
}

func newDownloadClientTable(schemaName, tableName, alias string) *DownloadClientTable {
	return &DownloadClientTable{
		downloadClientTable: newDownloadClientTableImpl(schemaName, tableName, alias),
		EXCLUDED:            newDownloadClientTableImpl("", "excluded", ""),
	}
}

func newDownloadClientTableImpl(schemaName, tableName, alias string) downloadClientTable {
	var (
		IDColumn             = sqlite.IntegerColumn("id")
		TypeColumn           = sqlite.StringColumn("type")
		ImplementationColumn = sqlite.StringColumn("implementation")
		SchemeColumn         = sqlite.StringColumn("scheme")
		HostColumn           = sqlite.StringColumn("host")
		PortColumn           = sqlite.IntegerColumn("port")
		DirectoryColumn      = sqlite.StringColumn("directory")
		allColumns           = sqlite.ColumnList{IDColumn, TypeColumn, ImplementationColumn, SchemeColumn, HostColumn, PortColumn, DirectoryColumn}
		mutableColumns       = sqlite.ColumnList{TypeColumn, ImplementationColumn, SchemeColumn, HostColumn, PortColumn, DirectoryColumn}
	)

	return downloadClientTable{
		Table: sqlite.NewTable(schemaName, tableName, alias, allColumns...),

		//Columns
		ID:             IDColumn,
		Type:           TypeColumn,
		Implementation: ImplementationColumn,
		Scheme:         SchemeColumn,
		Host:           HostColumn,
		Port:           PortColumn,
		Directory:      DirectoryColumn,

		AllColumns:     allColumns,
		MutableColumns: mutableColumns,
	}
}
