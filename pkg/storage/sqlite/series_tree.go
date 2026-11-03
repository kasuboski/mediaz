package sqlite

import (
	"context"
	"fmt"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
)

// CreateSeriesTree persists a fully validated metadata tree in one transaction.
// All provider I/O and policy validation must be completed before calling.
func (s *SQLite) CreateSeriesTree(ctx context.Context, series sqlcdb.Series, state storage.SeriesState, seasons []sqlcdb.Season, episodes [][]sqlcdb.Episode) (int64, error) {
	if len(seasons) != len(episodes) {
		return 0, fmt.Errorf("season/episode tree length mismatch")
	}
	if err := storage.SeriesStateNew.Machine().ToState(state); err != nil {
		return 0, err
	}
	var id int64
	err := s.withTx(ctx, func(q *sqlcdb.Queries) error {
		var err error
		id, err = q.CreateSeries(ctx, sqlcdb.CreateSeriesParams{Path: series.Path, Monitored: series.Monitored, QualityProfileID: series.QualityProfileID, SeriesMetadataID: series.SeriesMetadataID, MonitorNewSeasons: series.MonitorNewSeasons})
		if err != nil {
			return err
		}
		if err = q.InsertSeriesTransition(ctx, sqlcdb.InsertSeriesTransitionParams{SeriesID: id, ToState: string(state), MostRecent: true, SortKey: 1}); err != nil {
			return err
		}
		for i, season := range seasons {
			sid, err := q.CreateSeason(ctx, sqlcdb.CreateSeasonParams{SeriesID: id, SeasonNumber: season.SeasonNumber, SeasonMetadataID: season.SeasonMetadataID, Monitored: season.Monitored, QualityProfileID: season.QualityProfileID})
			if err != nil {
				return err
			}
			if err = q.InsertSeasonTransition(ctx, sqlcdb.InsertSeasonTransitionParams{SeasonID: sid, ToState: string(storage.SeasonStateMissing), MostRecent: true, SortKey: 1}); err != nil {
				return err
			}
			for _, episode := range episodes[i] {
				eid, err := q.CreateEpisode(ctx, sqlcdb.CreateEpisodeParams{SeasonID: sid, EpisodeMetadataID: episode.EpisodeMetadataID, EpisodeNumber: episode.EpisodeNumber, Monitored: episode.Monitored, QualityProfileID: episode.QualityProfileID})
				if err != nil {
					return err
				}
				if err = q.InsertEpisodeTransition(ctx, sqlcdb.InsertEpisodeTransitionParams{EpisodeID: eid, ToState: string(storage.EpisodeStateMissing), MostRecent: true, SortKey: 1}); err != nil {
					return err
				}
			}
		}
		return nil
	})
	return id, err
}
