package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/model"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
)

// tvEpisodeFromRow converts a single-row episode result into the domain shape.
// The transition join is a LEFT JOIN, so transition columns may be NULL when
// an episode has no transitions yet.
func tvEpisodeFromRow(r sqlcdb.GetEpisodeRow) *storage.Episode {
	return &storage.Episode{
		Episode: model.Episode{
			ID:                int32(r.ID),
			SeasonID:          int32(r.SeasonID),
			EpisodeNumber:     int32(r.EpisodeNumber),
			Monitored:         int32(r.Monitored),
			EpisodeMetadataID: tvInt32Ptr(r.EpisodeMetadataID),
			EpisodeFileID:     tvInt32Ptr(r.EpisodeFileID),
		},
		State:                  storage.EpisodeState(tvStringFromNull(r.ToState)),
		DownloadID:             tvStringFromNull(r.DownloadID),
		DownloadClientID:       tvInt32(r.DownloadClientID),
		IsEntireSeasonDownload: r.IsEntireSeasonDownload.Bool && r.IsEntireSeasonDownload.Valid,
	}
}

// tvEpisodeFromListRow converts a list-episode result into the domain shape.
// The list query uses an INNER JOIN, so the state is always present.
func tvEpisodeFromListRow(r sqlcdb.ListEpisodesRow) *storage.Episode {
	return &storage.Episode{
		Episode: model.Episode{
			ID:                int32(r.ID),
			SeasonID:          int32(r.SeasonID),
			EpisodeNumber:     int32(r.EpisodeNumber),
			Monitored:         int32(r.Monitored),
			EpisodeMetadataID: tvInt32Ptr(r.EpisodeMetadataID),
			EpisodeFileID:     tvInt32Ptr(r.EpisodeFileID),
		},
		State:                  storage.EpisodeState(r.ToState),
		DownloadID:             tvStringFromNull(r.DownloadID),
		DownloadClientID:       tvInt32(r.DownloadClientID),
		IsEntireSeasonDownload: r.IsEntireSeasonDownload.Bool && r.IsEntireSeasonDownload.Valid,
	}
}

// tvEpisodeFromFileIDRow converts a GetEpisodeByEpisodeFileID result into the
// domain shape. Like the legacy statement it does not select
// is_entire_season_download, so that field stays false here.
func tvEpisodeFromFileIDRow(r sqlcdb.GetEpisodeByEpisodeFileIDRow) *storage.Episode {
	return &storage.Episode{
		Episode: model.Episode{
			ID:                int32(r.ID),
			SeasonID:          int32(r.SeasonID),
			EpisodeNumber:     int32(r.EpisodeNumber),
			Monitored:         int32(r.Monitored),
			EpisodeMetadataID: tvInt32Ptr(r.EpisodeMetadataID),
			EpisodeFileID:     tvInt32Ptr(r.EpisodeFileID),
		},
		State:            storage.EpisodeState(tvStringFromNull(r.ToState)),
		DownloadID:       tvStringFromNull(r.DownloadID),
		DownloadClientID: tvInt32(r.DownloadClientID),
	}
}

// CreateEpisode stores an episode and creates an initial transition state
func (s *SQLite) CreateEpisode(ctx context.Context, episode storage.Episode, initialState storage.EpisodeState) (int64, error) {
	if episode.State == "" {
		episode.State = storage.EpisodeStateNew
	}

	err := episode.Machine().ToState(initialState)
	if err != nil {
		return 0, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}

	// A NULL id lets SQLite assign a fresh AUTOINCREMENT rowid; a non-zero id
	// inserts explicitly or lands in the ON CONFLICT branch, matching the two
	// legacy insert column lists.
	inserted, err := s.txQueries(tx).CreateEpisode(ctx, sqlcdb.CreateEpisodeParams{
		ID:                tvNullID(episode.ID),
		SeasonID:          int64(episode.SeasonID),
		EpisodeNumber:     int64(episode.EpisodeNumber),
		Monitored:         int64(episode.Monitored),
		EpisodeMetadataID: tvNullInt64FromInt32Ptr(episode.EpisodeMetadataID),
		EpisodeFileID:     tvNullInt64FromInt32Ptr(episode.EpisodeFileID),
	})
	if err != nil {
		tx.Rollback()
		return 0, err
	}

	err = s.txQueries(tx).InsertEpisodeTransition(ctx, sqlcdb.InsertEpisodeTransitionParams{
		EpisodeID:  inserted,
		ToState:    string(initialState),
		FromState:  sql.NullString{},
		MostRecent: true,
		SortKey:    1,
	})
	if err != nil {
		tx.Rollback()
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		tx.Rollback()
		return 0, err
	}

	return inserted, nil
}

// GetEpisodeByID gets an episode by id
func (s *SQLite) GetEpisodeByID(ctx context.Context, id int64) (*storage.Episode, error) {
	return s.getEpisode(ctx, sqlcdb.GetEpisodeParams{ID: sql.NullInt64{Int64: id, Valid: true}})
}

func (s *SQLite) getEpisode(ctx context.Context, params sqlcdb.GetEpisodeParams) (*storage.Episode, error) {
	row, err := s.Queries.GetEpisode(ctx, params)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, storage.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get episode: %w", err)
	}

	return tvEpisodeFromRow(row), nil
}

// DeleteEpisode removes an episode by id
func (s *SQLite) DeleteEpisode(ctx context.Context, id int64) error {
	err := s.Queries.DeleteEpisode(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to delete episode: %w", err)
	}

	return nil
}

// ListEpisodes lists episodes constrained by the filter; a zero-value filter
// lists everything.
func (s *SQLite) ListEpisodes(ctx context.Context, filter storage.EpisodeFilter) ([]*storage.Episode, error) {
	params := sqlcdb.ListEpisodesParams{}
	if filter.SeasonID != nil {
		params.SeasonID = sql.NullInt64{Int64: *filter.SeasonID, Valid: true}
	}

	slots, err := tvStateSlotsFrom(tvStateStrings(filter.States))
	if err != nil {
		return nil, fmt.Errorf("failed to list episodes: %w", err)
	}
	params.StatesAny = slots.Any
	params.State2 = slots.S2
	params.State3 = slots.S3
	params.State4 = slots.S4

	rows, err := s.Queries.ListEpisodes(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to list episodes: %w", err)
	}

	episodes := make([]*storage.Episode, 0, len(rows))
	for _, r := range rows {
		episodes = append(episodes, tvEpisodeFromListRow(r))
	}

	return episodes, nil
}

// GetEpisodeByEpisodeFileID gets an episode by its associated file ID
func (s *SQLite) GetEpisodeByEpisodeFileID(ctx context.Context, fileID int64) (*storage.Episode, error) {
	row, err := s.Queries.GetEpisodeByEpisodeFileID(ctx, sql.NullInt64{Int64: fileID, Valid: true})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, storage.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get episode by file id: %w", err)
	}

	return tvEpisodeFromFileIDRow(row), nil
}

// UpdateEpisodeState updates the transition state of an episode
// Metadata is optional and can be nil
func (s *SQLite) UpdateEpisodeState(ctx context.Context, id int64, state storage.EpisodeState, metadata *storage.TransitionStateMetadata) error {
	episode, err := s.GetEpisodeByID(ctx, id)
	if err != nil {
		return err
	}

	err = episode.Machine().ToState(state)
	if err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	// Demote the current most recent transition; the query errors when the
	// episode has no transition, matching the legacy behavior.
	previousSortKey, err := s.txQueries(tx).DemoteEpisodeTransition(ctx, id)
	if err != nil {
		tx.Rollback()
		return err
	}

	transition := sqlcdb.InsertEpisodeTransitionParams{
		EpisodeID:  id,
		ToState:    string(state),
		FromState:  sql.NullString{},
		MostRecent: true,
		SortKey:    int64(int32(previousSortKey) + 1),
	}

	if metadata != nil {
		if metadata.DownloadClientID != nil {
			transition.DownloadClientID = sql.NullInt64{Int64: int64(*metadata.DownloadClientID), Valid: true}
		}
		if metadata.DownloadID != nil {
			transition.DownloadID = sql.NullString{String: *metadata.DownloadID, Valid: true}
		}
		if metadata.IsEntireSeasonDownload != nil {
			transition.IsEntireSeasonDownload = sql.NullBool{Bool: *metadata.IsEntireSeasonDownload, Valid: true}
		}
	}

	err = s.txQueries(tx).InsertEpisodeTransition(ctx, transition)
	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

// UpdateEpisodeEpisodeFileID updates the episode file id for an episode
func (s *SQLite) UpdateEpisodeEpisodeFileID(ctx context.Context, id int64, fileID int64) error {
	err := s.Queries.UpdateEpisodeEpisodeFileID(ctx, sqlcdb.UpdateEpisodeEpisodeFileIDParams{
		EpisodeFileID: sql.NullInt64{Int64: fileID, Valid: true},
		ID:            id,
	})
	if err != nil {
		return fmt.Errorf("failed to update episode file id: %w", err)
	}

	return nil
}

// LinkEpisodeMetadata links an episode with its season and episode metadata
func (s *SQLite) LinkEpisodeMetadata(ctx context.Context, episodeID int64, seasonID int32, episodeMetadataID int32) error {
	return s.Queries.LinkEpisodeMetadata(ctx, sqlcdb.LinkEpisodeMetadataParams{
		SeasonID:          int64(seasonID),
		EpisodeMetadataID: sql.NullInt64{Int64: int64(episodeMetadataID), Valid: true},
		ID:                episodeID,
	})
}
