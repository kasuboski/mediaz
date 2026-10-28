package manager

import "github.com/kasuboski/mediaz/pkg/storage/sqlcdb"

// seriesReleaseIdentity extracts known provider identifiers while retaining TMDB
// identity even when external IDs are absent or malformed.
func seriesReleaseIdentity(metadata sqlcdb.SeriesMetadatum) SeriesReleaseFilterParams {
	params := SeriesReleaseFilterParams{TmdbID: metadata.TmdbID}
	if metadata.FirstAirDate.Valid && !metadata.FirstAirDate.Time.IsZero() {
		year := int32(metadata.FirstAirDate.Time.Year())
		params.Year = &year
	}
	if !metadata.ExternalIds.Valid {
		return params
	}

	externalIDs, err := DeserializeExternalIDs(&metadata.ExternalIds.String)
	if err != nil || externalIDs == nil {
		return params
	}
	if externalIDs.ImdbID != nil {
		params.IMDbID = *externalIDs.ImdbID
	}
	if externalIDs.TvdbID != nil {
		params.TVDBID = int64(*externalIDs.TvdbID)
	}
	return params
}
