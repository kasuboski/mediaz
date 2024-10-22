//
// Code generated by go-jet DO NOT EDIT.
//
// WARNING: Changes to this file may cause incorrect behavior
// and will be lost if the code is regenerated
//

package table

// UseSchema sets a new schema name for all generated table SQL builder types. It is recommended to invoke
// this method only once at the beginning of the program.
func UseSchema(schema string) {
	DownloadClient = DownloadClient.FromSchema(schema)
	Indexer = Indexer.FromSchema(schema)
	Movie = Movie.FromSchema(schema)
	MovieFile = MovieFile.FromSchema(schema)
	MovieMetadata = MovieMetadata.FromSchema(schema)
	QualityDefinition = QualityDefinition.FromSchema(schema)
	QualityProfile = QualityProfile.FromSchema(schema)
	QualityProfileItem = QualityProfileItem.FromSchema(schema)
}
