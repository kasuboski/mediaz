package server

import (
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"github.com/stretchr/testify/require"
)

func TestPersistenceResponseBoundary(t *testing.T) {
	t.Run("movie nulls and internal download fields", func(t *testing.T) {
		row := sqlcdb.GetMovieByIDRow{Movie: sqlcdb.Movie{ID: 7, QualityProfileID: 2}, State: "missing", DownloadID: sql.NullString{String: "internal-download", Valid: true}, DownloadClientID: sql.NullInt64{Int64: 4, Valid: true}}
		raw, err := json.Marshal(movieDTO(&row))
		require.NoError(t, err)
		require.JSONEq(t, `{"ID":7,"Path":null,"Monitored":0,"QualityProfileID":2,"Added":null,"Tags":null,"AddOptions":null,"MovieFileID":null,"MinimumAvailability":0,"MovieMetadataID":null,"LastSearchTime":null,"state":"missing"}`, string(raw))
	})
	t.Run("download credentials never leave seam", func(t *testing.T) {
		row := sqlcdb.DownloadClient{ID: 1, Type: "sabnzbd", Implementation: "SABNZBD", Scheme: "https", Host: "sab.local", Port: 8080, ApiKey: sql.NullString{String: "secret-value", Valid: true}}
		raw, err := json.Marshal(downloadClientDTO(row))
		require.NoError(t, err)
		require.JSONEq(t, `{"ID":1,"Type":"sabnzbd","Implementation":"SABNZBD","Scheme":"https","Host":"sab.local","Port":8080}`, string(raw))
	})
	t.Run("scalar credentials decoded at input seam", func(t *testing.T) {
		var request downloadClientRequest
		require.NoError(t, json.Unmarshal([]byte(`{"Type":"sabnzbd","APIKey":"scalar-secret"}`), &request))
		require.Equal(t, sqlcdb.DownloadClient{Type: "sabnzbd", ApiKey: sql.NullString{String: "scalar-secret", Valid: true}}, request.row())
	})
	t.Run("profile without items remains present", func(t *testing.T) {
		result := qualityProfileDTO(sqlcdb.QualityProfile{ID: 9, Name: "Empty", CutoffQualityID: sql.NullInt64{Int64: 2, Valid: true}, UpgradeAllowed: true}, nil)
		raw, err := json.Marshal(result)
		require.NoError(t, err)
		require.JSONEq(t, `{"name":"Empty","qualities":null,"id":9,"cutoff_quality_id":2,"upgradeAllowed":true}`, string(raw))
	})
}
