package manager

import "database/sql"

// ResolveTVQualityProfileID resolves episode > season > series inheritance.
// The source is one of episode, season, or series.
func ResolveTVQualityProfileID(seriesID int64, seasonOverride, episodeOverride sql.NullInt64) (int64, string) {
	if episodeOverride.Valid && episodeOverride.Int64 > 0 {
		return episodeOverride.Int64, "episode"
	}
	if seasonOverride.Valid && seasonOverride.Int64 > 0 {
		return seasonOverride.Int64, "season"
	}
	return seriesID, "series"
}
