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

var SubtitleFiles = newSubtitleFilesTable("", "SubtitleFiles", "")

type subtitleFilesTable struct {
	sqlite.Table

	// Columns
	ID           sqlite.ColumnInteger
	MovieId      sqlite.ColumnInteger
	MovieFileId  sqlite.ColumnInteger
	RelativePath sqlite.ColumnString
	Extension    sqlite.ColumnString
	Added        sqlite.ColumnTimestamp
	LastUpdated  sqlite.ColumnTimestamp
	Language     sqlite.ColumnInteger
	LanguageTags sqlite.ColumnString

	AllColumns     sqlite.ColumnList
	MutableColumns sqlite.ColumnList
}

type SubtitleFilesTable struct {
	subtitleFilesTable

	EXCLUDED subtitleFilesTable
}

// AS creates new SubtitleFilesTable with assigned alias
func (a SubtitleFilesTable) AS(alias string) *SubtitleFilesTable {
	return newSubtitleFilesTable(a.SchemaName(), a.TableName(), alias)
}

// Schema creates new SubtitleFilesTable with assigned schema name
func (a SubtitleFilesTable) FromSchema(schemaName string) *SubtitleFilesTable {
	return newSubtitleFilesTable(schemaName, a.TableName(), a.Alias())
}

// WithPrefix creates new SubtitleFilesTable with assigned table prefix
func (a SubtitleFilesTable) WithPrefix(prefix string) *SubtitleFilesTable {
	return newSubtitleFilesTable(a.SchemaName(), prefix+a.TableName(), a.TableName())
}

// WithSuffix creates new SubtitleFilesTable with assigned table suffix
func (a SubtitleFilesTable) WithSuffix(suffix string) *SubtitleFilesTable {
	return newSubtitleFilesTable(a.SchemaName(), a.TableName()+suffix, a.TableName())
}

func newSubtitleFilesTable(schemaName, tableName, alias string) *SubtitleFilesTable {
	return &SubtitleFilesTable{
		subtitleFilesTable: newSubtitleFilesTableImpl(schemaName, tableName, alias),
		EXCLUDED:           newSubtitleFilesTableImpl("", "excluded", ""),
	}
}

func newSubtitleFilesTableImpl(schemaName, tableName, alias string) subtitleFilesTable {
	var (
		IDColumn           = sqlite.IntegerColumn("Id")
		MovieIdColumn      = sqlite.IntegerColumn("MovieId")
		MovieFileIdColumn  = sqlite.IntegerColumn("MovieFileId")
		RelativePathColumn = sqlite.StringColumn("RelativePath")
		ExtensionColumn    = sqlite.StringColumn("Extension")
		AddedColumn        = sqlite.TimestampColumn("Added")
		LastUpdatedColumn  = sqlite.TimestampColumn("LastUpdated")
		LanguageColumn     = sqlite.IntegerColumn("Language")
		LanguageTagsColumn = sqlite.StringColumn("LanguageTags")
		allColumns         = sqlite.ColumnList{IDColumn, MovieIdColumn, MovieFileIdColumn, RelativePathColumn, ExtensionColumn, AddedColumn, LastUpdatedColumn, LanguageColumn, LanguageTagsColumn}
		mutableColumns     = sqlite.ColumnList{MovieIdColumn, MovieFileIdColumn, RelativePathColumn, ExtensionColumn, AddedColumn, LastUpdatedColumn, LanguageColumn, LanguageTagsColumn}
	)

	return subtitleFilesTable{
		Table: sqlite.NewTable(schemaName, tableName, alias, allColumns...),

		//Columns
		ID:           IDColumn,
		MovieId:      MovieIdColumn,
		MovieFileId:  MovieFileIdColumn,
		RelativePath: RelativePathColumn,
		Extension:    ExtensionColumn,
		Added:        AddedColumn,
		LastUpdated:  LastUpdatedColumn,
		Language:     LanguageColumn,
		LanguageTags: LanguageTagsColumn,

		AllColumns:     allColumns,
		MutableColumns: mutableColumns,
	}
}
