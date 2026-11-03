package server

import (
	"time"

	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
)

// These DTOs preserve the historical HTTP field names and nullability without
// exposing sqlc's persistence representation (or stored credentials).
type movieResponse struct {
	ID                  int64      `json:"ID"`
	Path                *string    `json:"Path"`
	Monitored           int64      `json:"Monitored"`
	QualityProfileID    int64      `json:"QualityProfileID"`
	Added               *time.Time `json:"Added"`
	Tags                *string    `json:"Tags"`
	AddOptions          *string    `json:"AddOptions"`
	MovieFileID         *int64     `json:"MovieFileID"`
	MinimumAvailability int64      `json:"MinimumAvailability"`
	MovieMetadataID     *int64     `json:"MovieMetadataID"`
	LastSearchTime      *time.Time `json:"LastSearchTime"`
	State               string     `json:"state"`
}

func movieDTO(row *sqlcdb.GetMovieByIDRow) *movieResponse {
	if row == nil {
		return nil
	}
	m := row.Movie
	return &movieResponse{ID: m.ID, Path: nullable(m.Path.String, m.Path.Valid), Monitored: m.Monitored, QualityProfileID: m.QualityProfileID, Added: nullable(m.Added.Time, m.Added.Valid), Tags: nullable(m.Tags.String, m.Tags.Valid), AddOptions: nullable(m.AddOptions.String, m.AddOptions.Valid), MovieFileID: nullable(m.MovieFileID.Int64, m.MovieFileID.Valid), MinimumAvailability: m.MinimumAvailability, MovieMetadataID: nullable(m.MovieMetadataID.Int64, m.MovieMetadataID.Valid), LastSearchTime: nullable(m.LastSearchTime.Time, m.LastSearchTime.Valid), State: row.State}
}

type seriesResponse struct {
	ID                int64      `json:"ID"`
	Path              *string    `json:"Path"`
	Monitored         int64      `json:"Monitored"`
	Added             *time.Time `json:"Added"`
	QualityProfileID  int64      `json:"QualityProfileID"`
	SeriesMetadataID  *int64     `json:"SeriesMetadataID"`
	LastSearchTime    *time.Time `json:"LastSearchTime"`
	MonitorNewSeasons int64      `json:"MonitorNewSeasons"`
	State             string     `json:"state"`
}

func seriesDTO(row *sqlcdb.GetSeriesRow) *seriesResponse {
	if row == nil {
		return nil
	}
	m := row.Series
	return &seriesResponse{ID: m.ID, Path: nullable(m.Path.String, m.Path.Valid), Monitored: m.Monitored, Added: nullable(m.Added.Time, m.Added.Valid), QualityProfileID: m.QualityProfileID, SeriesMetadataID: nullable(m.SeriesMetadataID.Int64, m.SeriesMetadataID.Valid), LastSearchTime: nullable(m.LastSearchTime.Time, m.LastSearchTime.Valid), MonitorNewSeasons: m.MonitorNewSeasons, State: row.State}
}

// No APIKey field: credentials are write-only at the HTTP boundary.
type downloadClientResponse struct {
	ID             int64  `json:"ID"`
	Type           string `json:"Type"`
	Implementation string `json:"Implementation"`
	Scheme         string `json:"Scheme"`
	Host           string `json:"Host"`
	Port           int64  `json:"Port"`
}

func downloadClientDTO(c sqlcdb.DownloadClient) downloadClientResponse {
	return downloadClientResponse{ID: c.ID, Type: c.Type, Implementation: c.Implementation, Scheme: c.Scheme, Host: c.Host, Port: c.Port}
}
func downloadClientsDTO(clients []sqlcdb.DownloadClient) []downloadClientResponse {
	if clients == nil {
		return nil
	}
	result := make([]downloadClientResponse, len(clients))
	for i, c := range clients {
		result[i] = downloadClientDTO(c)
	}
	return result
}

type qualityDefinitionResponse struct {
	ID            int64   `json:"ID"`
	QualityID     int64   `json:"QualityID"`
	Name          string  `json:"Name"`
	PreferredSize float64 `json:"PreferredSize"`
	MinSize       float64 `json:"MinSize"`
	MaxSize       float64 `json:"MaxSize"`
	MediaType     string  `json:"MediaType"`
}

func qualityDefinitionDTO(q sqlcdb.QualityDefinition) qualityDefinitionResponse {
	return qualityDefinitionResponse{ID: q.ID, QualityID: q.QualityID, Name: q.Name, PreferredSize: q.PreferredSize, MinSize: q.MinSize, MaxSize: q.MaxSize, MediaType: q.MediaType}
}
func qualityDefinitionsDTO(rows []sqlcdb.QualityDefinition) []qualityDefinitionResponse {
	if rows == nil {
		return nil
	}
	result := make([]qualityDefinitionResponse, len(rows))
	for i, q := range rows {
		result[i] = qualityDefinitionDTO(q)
	}
	return result
}

type profileQualityResponse struct {
	ID            int64   `json:"id"`
	Name          string  `json:"name"`
	MediaType     string  `json:"type"`
	PreferredSize float64 `json:"preferredSize"`
	MinSize       float64 `json:"minSize"`
	MaxSize       float64 `json:"maxSize"`
}
type qualityProfileResponse struct {
	Name            string                   `json:"name"`
	Qualities       []profileQualityResponse `json:"qualities"`
	ID              int64                    `json:"id"`
	CutoffQualityID *int64                   `json:"cutoff_quality_id,omitempty"`
	UpgradeAllowed  bool                     `json:"upgradeAllowed"`
}

func qualityProfileDTO(profile sqlcdb.QualityProfile, rows []sqlcdb.ListQualityProfileRowsRow) qualityProfileResponse {
	result := qualityProfileResponse{Name: profile.Name, ID: profile.ID, CutoffQualityID: nullable(profile.CutoffQualityID.Int64, profile.CutoffQualityID.Valid), UpgradeAllowed: profile.UpgradeAllowed}
	for _, row := range rows {
		if row.QualityProfile.ID != profile.ID {
			continue
		}
		q := row.QualityDefinition
		result.Qualities = append(result.Qualities, profileQualityResponse{ID: q.ID, Name: q.Name, MediaType: q.MediaType, PreferredSize: q.PreferredSize, MinSize: q.MinSize, MaxSize: q.MaxSize})
	}
	return result
}

// nullable is confined to serialization; persistence and business logic retain
// the generated sql.Null* fields. Invalid values serialize as JSON null.
func nullable[T any](value T, valid bool) *T {
	if !valid {
		return nil
	}
	return &value
}
