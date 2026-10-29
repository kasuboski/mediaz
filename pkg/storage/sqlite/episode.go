package sqlite

import (
	"context"
	"database/sql"
	"sort"

	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
)

// CreateEpisode stores an episode and creates an initial transition state. A
// zero episode.ID lets SQLite assign a fresh autoincrement id; a non-zero id
// upserts by id.
func (s *SQLite) CreateEpisode(ctx context.Context, episode sqlcdb.Episode, initialState storage.EpisodeState) (int64, error) {
	if err := storage.EpisodeStateNew.Machine().ToState(initialState); err != nil {
		return 0, err
	}

	var inserted int64
	err := s.withTx(ctx, func(q *sqlcdb.Queries) error {
		var err error
		inserted, err = q.CreateEpisode(ctx, sqlcdb.CreateEpisodeParams{
			ID:                sql.NullInt64{Int64: episode.ID, Valid: episode.ID != 0},
			SeasonID:          episode.SeasonID,
			EpisodeNumber:     episode.EpisodeNumber,
			Monitored:         episode.Monitored,
			EpisodeMetadataID: episode.EpisodeMetadataID,
			EpisodeFileID:     episode.EpisodeFileID,
		})
		if err != nil {
			return err
		}

		current, err := q.GetEpisodeByID(ctx, inserted)
		if err != nil {
			return err
		}
		// Upserts update the entity, never reset its transition history.
		if current.State.Valid {
			return nil
		}

		return q.InsertEpisodeTransition(ctx, sqlcdb.InsertEpisodeTransitionParams{
			EpisodeID:  inserted,
			ToState:    string(initialState),
			MostRecent: true,
			SortKey:    1,
		})
	})
	if err != nil {
		return 0, err
	}

	return inserted, nil
}

// GetEpisodeByID gets an episode by id
func (s *SQLite) GetEpisodeByID(ctx context.Context, id int64) (sqlcdb.GetEpisodeByIDRow, error) {
	row, err := s.Queries.GetEpisodeByID(ctx, id)
	if err != nil {
		return sqlcdb.GetEpisodeByIDRow{}, mapQueryErr(err, "failed to get episode")
	}
	return row, nil
}

// DeleteEpisode removes an episode by id
func (s *SQLite) DeleteEpisode(ctx context.Context, id int64) error {
	if err := s.Queries.DeleteEpisode(ctx, id); err != nil {
		return err
	}
	return nil
}

// ListEpisodes lists episodes constrained by the filter; a zero-value filter
// lists everything. The States filter accepts any number of states.
func (s *SQLite) ListEpisodes(ctx context.Context, filter storage.EpisodeFilter) ([]sqlcdb.ListEpisodesRow, error) {
	params := sqlcdb.ListEpisodesParams{
		States: make([]string, 0, len(filter.States)),
	}
	if filter.SeasonID != nil {
		params.SeasonID = sql.NullInt64{Int64: *filter.SeasonID, Valid: true}
	}
	for _, state := range filter.States {
		params.States = append(params.States, string(state))
	}
	if len(params.States) > 0 {
		params.StatesAny = sql.NullString{String: params.States[0], Valid: true}
	}

	rows, err := s.Queries.ListEpisodes(ctx, params)
	if err != nil {
		return nil, mapQueryErr(err, "failed to list episodes")
	}
	return rows, nil
}

// ListEpisodesBySeasonIDs returns episodes for multiple seasons in stable season/episode order.
func (s *SQLite) ListEpisodesBySeasonIDs(ctx context.Context, seasonIDs []int64) ([]sqlcdb.ListEpisodesBySeasonIDsRow, error) {
	seasonIDs = uniqueSortedIDs(seasonIDs)
	rows := make([]sqlcdb.ListEpisodesBySeasonIDsRow, 0)
	for start := 0; start < len(seasonIDs); start += sqliteBindLimit {
		end := min(start+sqliteBindLimit, len(seasonIDs))
		batch, err := s.Queries.ListEpisodesBySeasonIDs(ctx, seasonIDs[start:end])
		if err != nil {
			return nil, mapQueryErr(err, "failed to list episodes by season ids")
		}
		rows = append(rows, batch...)
	}
	sort.Slice(rows, func(i, j int) bool {
		a, b := rows[i].Episode, rows[j].Episode
		if a.SeasonID != b.SeasonID {
			return a.SeasonID < b.SeasonID
		}
		if a.EpisodeNumber != b.EpisodeNumber {
			return a.EpisodeNumber < b.EpisodeNumber
		}
		return a.ID < b.ID
	})
	return rows, nil
}

// GetEpisodeByEpisodeFileID gets an episode by its associated file ID
func (s *SQLite) GetEpisodeByEpisodeFileID(ctx context.Context, fileID int64) (sqlcdb.GetEpisodeByEpisodeFileIDRow, error) {
	row, err := s.Queries.GetEpisodeByEpisodeFileID(ctx, sql.NullInt64{Int64: fileID, Valid: true})
	if err != nil {
		return sqlcdb.GetEpisodeByEpisodeFileIDRow{}, mapQueryErr(err, "failed to get episode by file id")
	}
	return row, nil
}

// UpdateEpisodeState updates the transition state of an episode. Metadata is
// optional and can be nil. The demote+insert pair runs in one transaction; on
// any failure nothing is written.
func (s *SQLite) UpdateEpisodeState(ctx context.Context, id int64, state storage.EpisodeState, metadata *storage.TransitionStateMetadata) error {
	return s.withTx(ctx, func(q *sqlcdb.Queries) error {
		episode, err := q.GetEpisodeByID(ctx, id)
		if err != nil {
			return mapQueryErr(err, "failed to get episode")
		}
		if err := storage.EpisodeState(episode.State.String).Machine().ToState(state); err != nil {
			return err
		}

		// Demote the current most recent transition; the query errors when the
		// episode has no transition, matching the legacy behavior.
		previousSortKey, err := q.DemoteEpisodeTransition(ctx, id)
		if err != nil {
			return err
		}

		transition := sqlcdb.InsertEpisodeTransitionParams{
			EpisodeID:  id,
			FromState:  episode.State,
			ToState:    string(state),
			MostRecent: true,
			SortKey:    previousSortKey + 1,
		}
		if metadata != nil {
			if metadata.DownloadClientID != nil {
				transition.DownloadClientID = sql.NullInt64{Int64: *metadata.DownloadClientID, Valid: true}
			}
			if metadata.DownloadID != nil {
				transition.DownloadID = sql.NullString{String: *metadata.DownloadID, Valid: true}
			}
			if metadata.IsEntireSeasonDownload != nil {
				transition.IsEntireSeasonDownload = sql.NullBool{Bool: *metadata.IsEntireSeasonDownload, Valid: true}
			}
		}

		return q.InsertEpisodeTransition(ctx, transition)
	})
}

// UpdateEpisodeEpisodeFileID updates the episode file id for an episode
func (s *SQLite) UpdateEpisodeEpisodeFileID(ctx context.Context, id int64, fileID int64) error {
	return s.Queries.UpdateEpisodeEpisodeFileID(ctx, sqlcdb.UpdateEpisodeEpisodeFileIDParams{
		EpisodeFileID: sql.NullInt64{Int64: fileID, Valid: true},
		ID:            id,
	})
}

// LinkEpisodeMetadata links an episode with its season and episode metadata
func (s *SQLite) LinkEpisodeMetadata(ctx context.Context, episodeID int64, seasonID int64, episodeMetadataID int64) error {
	return s.Queries.LinkEpisodeMetadata(ctx, sqlcdb.LinkEpisodeMetadataParams{
		SeasonID:          seasonID,
		EpisodeMetadataID: sql.NullInt64{Int64: episodeMetadataID, Valid: true},
		ID:                episodeID,
	})
}
