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

var MovieFiles = newMovieFilesTable("", "MovieFiles", "")

type movieFilesTable struct {
	sqlite.Table

	// Columns
	ID               sqlite.ColumnInteger
	MovieId          sqlite.ColumnInteger
	Quality          sqlite.ColumnString
	Size             sqlite.ColumnInteger
	DateAdded        sqlite.ColumnTimestamp
	SceneName        sqlite.ColumnString
	MediaInfo        sqlite.ColumnString
	ReleaseGroup     sqlite.ColumnString
	RelativePath     sqlite.ColumnString
	Edition          sqlite.ColumnString
	Languages        sqlite.ColumnString
	IndexerFlags     sqlite.ColumnInteger
	OriginalFilePath sqlite.ColumnString

	AllColumns     sqlite.ColumnList
	MutableColumns sqlite.ColumnList
}

type MovieFilesTable struct {
	movieFilesTable

	EXCLUDED movieFilesTable
}

// AS creates new MovieFilesTable with assigned alias
func (a MovieFilesTable) AS(alias string) *MovieFilesTable {
	return newMovieFilesTable(a.SchemaName(), a.TableName(), alias)
}

// Schema creates new MovieFilesTable with assigned schema name
func (a MovieFilesTable) FromSchema(schemaName string) *MovieFilesTable {
	return newMovieFilesTable(schemaName, a.TableName(), a.Alias())
}

// WithPrefix creates new MovieFilesTable with assigned table prefix
func (a MovieFilesTable) WithPrefix(prefix string) *MovieFilesTable {
	return newMovieFilesTable(a.SchemaName(), prefix+a.TableName(), a.TableName())
}

// WithSuffix creates new MovieFilesTable with assigned table suffix
func (a MovieFilesTable) WithSuffix(suffix string) *MovieFilesTable {
	return newMovieFilesTable(a.SchemaName(), a.TableName()+suffix, a.TableName())
}

func newMovieFilesTable(schemaName, tableName, alias string) *MovieFilesTable {
	return &MovieFilesTable{
		movieFilesTable: newMovieFilesTableImpl(schemaName, tableName, alias),
		EXCLUDED:        newMovieFilesTableImpl("", "excluded", ""),
	}
}

func newMovieFilesTableImpl(schemaName, tableName, alias string) movieFilesTable {
	var (
		IDColumn               = sqlite.IntegerColumn("Id")
		MovieIdColumn          = sqlite.IntegerColumn("MovieId")
		QualityColumn          = sqlite.StringColumn("Quality")
		SizeColumn             = sqlite.IntegerColumn("Size")
		DateAddedColumn        = sqlite.TimestampColumn("DateAdded")
		SceneNameColumn        = sqlite.StringColumn("SceneName")
		MediaInfoColumn        = sqlite.StringColumn("MediaInfo")
		ReleaseGroupColumn     = sqlite.StringColumn("ReleaseGroup")
		RelativePathColumn     = sqlite.StringColumn("RelativePath")
		EditionColumn          = sqlite.StringColumn("Edition")
		LanguagesColumn        = sqlite.StringColumn("Languages")
		IndexerFlagsColumn     = sqlite.IntegerColumn("IndexerFlags")
		OriginalFilePathColumn = sqlite.StringColumn("OriginalFilePath")
		allColumns             = sqlite.ColumnList{IDColumn, MovieIdColumn, QualityColumn, SizeColumn, DateAddedColumn, SceneNameColumn, MediaInfoColumn, ReleaseGroupColumn, RelativePathColumn, EditionColumn, LanguagesColumn, IndexerFlagsColumn, OriginalFilePathColumn}
		mutableColumns         = sqlite.ColumnList{MovieIdColumn, QualityColumn, SizeColumn, DateAddedColumn, SceneNameColumn, MediaInfoColumn, ReleaseGroupColumn, RelativePathColumn, EditionColumn, LanguagesColumn, IndexerFlagsColumn, OriginalFilePathColumn}
	)

	return movieFilesTable{
		Table: sqlite.NewTable(schemaName, tableName, alias, allColumns...),

		//Columns
		ID:               IDColumn,
		MovieId:          MovieIdColumn,
		Quality:          QualityColumn,
		Size:             SizeColumn,
		DateAdded:        DateAddedColumn,
		SceneName:        SceneNameColumn,
		MediaInfo:        MediaInfoColumn,
		ReleaseGroup:     ReleaseGroupColumn,
		RelativePath:     RelativePathColumn,
		Edition:          EditionColumn,
		Languages:        LanguagesColumn,
		IndexerFlags:     IndexerFlagsColumn,
		OriginalFilePath: OriginalFilePathColumn,

		AllColumns:     allColumns,
		MutableColumns: mutableColumns,
	}
}