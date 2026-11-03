package manager

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/kasuboski/mediaz/pkg/storage"
)

// SeriesSelectionPatch contains sparse, independent updates; child null means inherit.
type SeriesSelectionPatch struct {
	Monitored         *bool                    `json:"monitored,omitempty"`
	MonitorNewSeasons *bool                    `json:"monitorNewSeasons,omitempty"`
	QualityProfileID  *int64                   `json:"qualityProfileID,omitempty"`
	Seasons           []SeasonSelectionUpdate  `json:"seasons,omitempty"`
	Episodes          []EpisodeSelectionUpdate `json:"episodes,omitempty"`
}
type SeasonSelectionUpdate struct {
	ID               int64   `json:"id"`
	Monitored        *bool   `json:"monitored,omitempty"`
	QualityProfileID **int64 `json:"qualityProfileID,omitempty"`
}
type EpisodeSelectionUpdate struct {
	ID               int64   `json:"id"`
	Monitored        *bool   `json:"monitored,omitempty"`
	QualityProfileID **int64 `json:"qualityProfileID,omitempty"`
}

func rejectNull(data []byte, keys ...string) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	if fields == nil {
		return fmt.Errorf("%w: expected object", ErrValidation)
	}
	for _, key := range keys {
		if raw, ok := fields[key]; ok && string(raw) == "null" {
			return fmt.Errorf("%w: %s cannot be null", ErrValidation, key)
		}
	}
	return nil
}
func (p *SeriesSelectionPatch) UnmarshalJSON(data []byte) error {
	type plain SeriesSelectionPatch
	if err := rejectNull(data, "monitored", "monitorNewSeasons", "qualityProfileID", "seasons", "episodes"); err != nil {
		return err
	}
	return json.Unmarshal(data, (*plain)(p))
}
func decodeChild(data []byte, target any, quality ***int64) error {
	if err := rejectNull(data, "monitored", "id"); err != nil {
		return err
	}
	if err := json.Unmarshal(data, target); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	if raw, ok := fields["qualityProfileID"]; ok && string(raw) == "null" {
		var inherit *int64
		*quality = &inherit
	}
	return nil
}
func (p *SeasonSelectionUpdate) UnmarshalJSON(data []byte) error {
	type plain SeasonSelectionUpdate
	return decodeChild(data, (*plain)(p), &p.QualityProfileID)
}
func (p *EpisodeSelectionUpdate) UnmarshalJSON(data []byte) error {
	type plain EpisodeSelectionUpdate
	return decodeChild(data, (*plain)(p), &p.QualityProfileID)
}

func (s SeriesService) UpdateSelection(ctx context.Context, id int64, patch SeriesSelectionPatch) (*TVDetailResult, error) {
	unlock := storage.LockTVAdmission(id)
	defer unlock()
	if id <= 0 {
		return nil, fmt.Errorf("%w: invalid series id", ErrValidation)
	}
	p := storage.SeriesSelectionPatch{SeriesMonitored: patch.Monitored, MonitorNewSeasons: patch.MonitorNewSeasons, QualityProfileID: patch.QualityProfileID}
	if patch.QualityProfileID != nil {
		if err := s.validateTVProfile(ctx, *patch.QualityProfileID); err != nil {
			return nil, err
		}
	}
	convert := func(v **int64) (*int64, error) {
		if v == nil {
			return nil, nil
		}
		if *v == nil {
			return ptrInt64(0), nil
		}
		if err := s.validateTVProfile(ctx, **v); err != nil {
			return nil, err
		}
		return *v, nil
	}
	seen := map[int64]bool{}
	for _, item := range patch.Seasons {
		if item.ID <= 0 || seen[item.ID] {
			return nil, fmt.Errorf("%w: invalid or duplicate season id", ErrValidation)
		}
		seen[item.ID] = true
		q, err := convert(item.QualityProfileID)
		if err != nil {
			return nil, err
		}
		p.Seasons = append(p.Seasons, storage.SeasonSelectionPatch{ID: item.ID, Monitored: item.Monitored, QualityProfileID: q})
	}
	seen = map[int64]bool{}
	for _, item := range patch.Episodes {
		if item.ID <= 0 || seen[item.ID] {
			return nil, fmt.Errorf("%w: invalid or duplicate episode id", ErrValidation)
		}
		seen[item.ID] = true
		q, err := convert(item.QualityProfileID)
		if err != nil {
			return nil, err
		}
		p.Episodes = append(p.Episodes, storage.EpisodeSelectionPatch{ID: item.ID, Monitored: item.Monitored, QualityProfileID: q})
	}
	if err := s.seriesStorage.ApplySeriesSelectionPatch(ctx, id, p); err != nil {
		return nil, err
	}
	row, err := s.seriesStorage.GetSeriesByID(ctx, id)
	if err != nil {
		return nil, err
	}
	meta, err := s.seriesMetaStorage.GetSeriesMetadataByID(ctx, row.Series.SeriesMetadataID.Int64)
	if err != nil {
		return nil, err
	}
	return s.GetTVDetailByTMDBID(ctx, meta.TmdbID)
}
func ptrInt64(v int64) *int64 { return &v }
func (s SeriesService) validateTVProfile(ctx context.Context, id int64) error {
	if id <= 0 {
		return fmt.Errorf("%w: quality profile id must be positive", ErrValidation)
	}
	p, err := s.qualityService.GetQualityProfile(ctx, id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return fmt.Errorf("%w: unknown quality profile %d", ErrValidation, id)
		}
		return err
	}
	rows, err := s.qualityService.ListQualityProfileRows(ctx, id)
	if err != nil {
		return err
	}
	if p.ID == 0 || len(rows) == 0 {
		return fmt.Errorf("%w: profile %d is not episode-compatible", ErrValidation, id)
	}
	for _, row := range rows {
		if row.QualityDefinition.MediaType != "episode" {
			return fmt.Errorf("%w: profile %d is not episode-compatible", ErrValidation, id)
		}
	}
	return nil
}
func resolveInheritedProfile(seriesID int64, seasonOverride, episodeOverride sql.NullInt64) (int64, string) {
	return ResolveTVQualityProfileID(seriesID, seasonOverride, episodeOverride)
}
