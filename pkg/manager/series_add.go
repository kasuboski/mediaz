package manager

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"time"
)

var ErrSelectionStale = errors.New("selection references unknown metadata ordinal")

// detailSeasons overlays local policy/state onto the complete cached metadata tree.
func (s SeriesService) detailSeasons(ctx context.Context, metadataID, seriesID int64) ([]SeasonResult, error) {
	tree, err := s.previewSeasons(ctx, metadataID)
	if err != nil {
		return nil, err
	}
	local, err := s.getSeasonsWithEpisodes(ctx, seriesID)
	if err != nil {
		return nil, err
	}
	byNumber := map[int64]SeasonResult{}
	for _, season := range local {
		byNumber[season.Number] = season
	}
	for i, season := range tree {
		stored, ok := byNumber[season.Number]
		if !ok {
			continue
		}
		children := map[int64]EpisodeResult{}
		for _, child := range stored.Episodes {
			children[child.Number] = child
		}
		for j, child := range season.Episodes {
			if v, ok := children[child.Number]; ok {
				season.Episodes[j] = v
			}
		}
		stored.Episodes = season.Episodes
		stored.EpisodeCount = int32(len(season.Episodes))
		tree[i] = stored
	}
	return tree, nil
}

func (s SeriesService) AddSeriesToLibrary(ctx context.Context, r AddSeriesRequest) (*sqlcdb.GetSeriesRow, error) {
	if r.TMDBID <= 0 {
		return nil, fmt.Errorf("%w: tmdbID must be positive", ErrValidation)
	}
	meta, err := s.metadataProvider.GetSeriesMetadata(ctx, r.TMDBID)
	if err != nil {
		return nil, err
	}
	if existing, err := s.seriesStorage.GetSeriesByMetadataID(ctx, meta.ID); err == nil {
		return &existing, nil
	} else if !errors.Is(err, storage.ErrNotFound) {
		return nil, err
	}
	if err = s.validateTVProfile(ctx, r.QualityProfileID); err != nil {
		return nil, err
	}
	mode := "all"
	choices := map[int64]SeasonAddSelection{}
	if r.Selection != nil {
		mode = r.Selection.Mode
		if mode != "all" && mode != "none" && mode != "custom" {
			return nil, fmt.Errorf("%w: invalid selection mode", ErrValidation)
		}
		if mode != "custom" && r.Selection.Seasons != nil {
			return nil, fmt.Errorf("%w: seasons only allowed with custom", ErrValidation)
		}
		if mode == "custom" && r.Selection.Seasons == nil {
			return nil, fmt.Errorf("%w: custom requires seasons", ErrValidation)
		}
		for _, choice := range r.Selection.Seasons {
			if _, ok := choices[choice.SeasonNumber]; ok {
				return nil, fmt.Errorf("%w: duplicate season ordinal", ErrValidation)
			}
			choices[choice.SeasonNumber] = choice
		}
	}
	sms, err := s.seriesMetaStorage.ListSeasonMetadataBySeriesMetadataID(ctx, meta.ID)
	if err != nil {
		return nil, err
	}
	seasons := make([]sqlcdb.Season, 0, len(sms))
	episodes := make([][]sqlcdb.Episode, 0, len(sms))
	quality := func(v *int64) (sql.NullInt64, error) {
		if v == nil {
			return sql.NullInt64{}, nil
		}
		if err := s.validateTVProfile(ctx, *v); err != nil {
			return sql.NullInt64{}, err
		}
		return sql.NullInt64{Int64: *v, Valid: true}, nil
	}
	for _, sm := range sms {
		choice, selected := choices[sm.Number]
		delete(choices, sm.Number)
		season := sqlcdb.Season{SeasonNumber: sm.Number, SeasonMetadataID: sql.NullInt64{Int64: sm.ID, Valid: true}}
		if mode == "all" || selected && choice.Monitored {
			season.Monitored = 1
		}
		season.QualityProfileID, err = quality(choice.QualityProfileID)
		if err != nil {
			return nil, err
		}
		ecs := map[int64]EpisodeAddSelection{}
		for _, ec := range choice.Episodes {
			if _, ok := ecs[ec.EpisodeNumber]; ok {
				return nil, fmt.Errorf("%w: duplicate episode ordinal", ErrValidation)
			}
			ecs[ec.EpisodeNumber] = ec
		}
		ems, err := s.seriesMetaStorage.ListEpisodeMetadataBySeasonMetadataID(ctx, sm.ID)
		if err != nil {
			return nil, err
		}
		children := make([]sqlcdb.Episode, 0, len(ems))
		for _, em := range ems {
			ec, selected := ecs[em.Number]
			delete(ecs, em.Number)
			child := sqlcdb.Episode{EpisodeNumber: em.Number, EpisodeMetadataID: sql.NullInt64{Int64: em.ID, Valid: true}}
			if mode == "all" || selected && ec.Monitored {
				child.Monitored = 1
			}
			child.QualityProfileID, err = quality(ec.QualityProfileID)
			if err != nil {
				return nil, err
			}
			children = append(children, child)
		}
		if len(ecs) > 0 {
			return nil, ErrSelectionStale
		}
		seasons = append(seasons, season)
		episodes = append(episodes, children)
	}
	if len(choices) > 0 {
		return nil, ErrSelectionStale
	}
	series := sqlcdb.Series{SeriesMetadataID: sql.NullInt64{Int64: meta.ID, Valid: true}, QualityProfileID: r.QualityProfileID, Monitored: 1, Path: sql.NullString{String: meta.Title, Valid: true}}
	if r.Monitored != nil && !*r.Monitored {
		series.Monitored = 0
	}
	if r.MonitorNewSeasons {
		series.MonitorNewSeasons = 1
	}
	var air *time.Time
	if meta.FirstAirDate.Valid {
		air = &meta.FirstAirDate.Time
	}
	id, err := s.seriesStorage.CreateSeriesTree(ctx, series, initialSeriesState(air), seasons, episodes)
	if err != nil {
		return nil, err
	}
	row, err := s.seriesStorage.GetSeriesByID(ctx, id)
	return &row, err
}

func (s SeriesService) previewSeasons(ctx context.Context, metadataID int64) ([]SeasonResult, error) {
	seasons, err := s.seriesMetaStorage.ListSeasonMetadataBySeriesMetadataID(ctx, metadataID)
	if err != nil {
		return nil, err
	}
	result := make([]SeasonResult, 0, len(seasons))
	for _, sm := range seasons {
		ems, err := s.seriesMetaStorage.ListEpisodeMetadataBySeasonMetadataID(ctx, sm.ID)
		if err != nil {
			return nil, err
		}
		season := SeasonResult{TMDBID: sm.TmdbID, Number: sm.Number, Title: sm.Title, EpisodeCount: int32(len(ems)), Episodes: make([]EpisodeResult, 0, len(ems))}
		if sm.Overview.Valid {
			v := sm.Overview.String
			season.Overview = &v
		}
		if sm.AirDate.Valid {
			v := sm.AirDate.Time.Format("2006-01-02")
			season.AirDate = &v
		}
		for _, em := range ems {
			child := buildEpisodeResult(sqlcdb.ListEpisodesRow{}, &em, 0, sm.Number)
			season.Episodes = append(season.Episodes, child)
		}
		result = append(result, season)
	}
	return result, nil
}
