package manager

import (
	"database/sql"
	"testing"
	"time"

	"github.com/kasuboski/mediaz/pkg/ptr"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/stretchr/testify/assert"
)

func TestSeriesReleaseIdentityFromMetadata(t *testing.T) {
	assert.Equal(t, SeriesReleaseFilterParams{TmdbID: 42, Year: ptr.To(int32(2014))}, seriesReleaseIdentity(sqlcdb.SeriesMetadatum{
		TmdbID: 42, FirstAirDate: sql.NullTime{Time: time.Date(2014, 10, 7, 0, 0, 0, 0, time.UTC), Valid: true},
	}))
	for _, tc := range []struct {
		name     string
		external sql.NullString
		want     SeriesReleaseFilterParams
	}{
		{"absent", sql.NullString{}, SeriesReleaseFilterParams{TmdbID: 42}},
		{"empty", sql.NullString{Valid: true}, SeriesReleaseFilterParams{TmdbID: 42}},
		{"malformed", sql.NullString{String: "{bad", Valid: true}, SeriesReleaseFilterParams{TmdbID: 42}},
		{"null", sql.NullString{String: "null", Valid: true}, SeriesReleaseFilterParams{TmdbID: 42}},
		{"null fields", sql.NullString{String: `{"imdb_id":null,"tvdb_id":null}`, Valid: true}, SeriesReleaseFilterParams{TmdbID: 42}},
		{"valid IDs", sql.NullString{String: `{"imdb_id":"tt0000123","tvdb_id":5}`, Valid: true}, SeriesReleaseFilterParams{TmdbID: 42, IMDbID: "tt0000123", TVDBID: 5}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, seriesReleaseIdentity(sqlcdb.SeriesMetadatum{TmdbID: 42, ExternalIds: tc.external}))
		})
	}
}
