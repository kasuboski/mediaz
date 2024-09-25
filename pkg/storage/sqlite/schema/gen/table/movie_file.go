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

var MovieFile = newMovieFileTable("", "movie_file", "")

type movieFileTable struct {
	sqlite.Table

	// Columns
	ID               sqlite.ColumnInteger
	MovieID          sqlite.ColumnInteger
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

type MovieFileTable struct {
	movieFileTable

	EXCLUDED movieFileTable
}

// AS creates new MovieFileTable with assigned alias
func (a MovieFileTable) AS(alias string) *MovieFileTable {
	return newMovieFileTable(a.SchemaName(), a.TableName(), alias)
}

// Schema creates new MovieFileTable with assigned schema name
func (a MovieFileTable) FromSchema(schemaName string) *MovieFileTable {
	return newMovieFileTable(schemaName, a.TableName(), a.Alias())
}

// WithPrefix creates new MovieFileTable with assigned table prefix
func (a MovieFileTable) WithPrefix(prefix string) *MovieFileTable {
	return newMovieFileTable(a.SchemaName(), prefix+a.TableName(), a.TableName())
}

// WithSuffix creates new MovieFileTable with assigned table suffix
func (a MovieFileTable) WithSuffix(suffix string) *MovieFileTable {
	return newMovieFileTable(a.SchemaName(), a.TableName()+suffix, a.TableName())
}

func newMovieFileTable(schemaName, tableName, alias string) *MovieFileTable {
	return &MovieFileTable{
		movieFileTable: newMovieFileTableImpl(schemaName, tableName, alias),
		EXCLUDED:       newMovieFileTableImpl("", "excluded", ""),
	}
}

func newMovieFileTableImpl(schemaName, tableName, alias string) movieFileTable {
	var (
		IDColumn               = sqlite.IntegerColumn("id")
		MovieIDColumn          = sqlite.IntegerColumn("movie_id")
		QualityColumn          = sqlite.StringColumn("quality")
		SizeColumn             = sqlite.IntegerColumn("size")
		DateAddedColumn        = sqlite.TimestampColumn("date_added")
		SceneNameColumn        = sqlite.StringColumn("scene_name")
		MediaInfoColumn        = sqlite.StringColumn("media_info")
		ReleaseGroupColumn     = sqlite.StringColumn("release_group")
		RelativePathColumn     = sqlite.StringColumn("relative_path")
		EditionColumn          = sqlite.StringColumn("edition")
		LanguagesColumn        = sqlite.StringColumn("languages")
		IndexerFlagsColumn     = sqlite.IntegerColumn("indexer_flags")
		OriginalFilePathColumn = sqlite.StringColumn("original_file_path")
		allColumns             = sqlite.ColumnList{IDColumn, MovieIDColumn, QualityColumn, SizeColumn, DateAddedColumn, SceneNameColumn, MediaInfoColumn, ReleaseGroupColumn, RelativePathColumn, EditionColumn, LanguagesColumn, IndexerFlagsColumn, OriginalFilePathColumn}
		mutableColumns         = sqlite.ColumnList{MovieIDColumn, QualityColumn, SizeColumn, DateAddedColumn, SceneNameColumn, MediaInfoColumn, ReleaseGroupColumn, RelativePathColumn, EditionColumn, LanguagesColumn, IndexerFlagsColumn, OriginalFilePathColumn}
	)

	return movieFileTable{
		Table: sqlite.NewTable(schemaName, tableName, alias, allColumns...),

		//Columns
		ID:               IDColumn,
		MovieID:          MovieIDColumn,
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