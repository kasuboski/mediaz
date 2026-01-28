package sqlite

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-jet/jet/v2/qrm"
	"github.com/go-jet/jet/v2/sqlite"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/table"
)

// CreateEpisode stores an episode and creates an initial transition state
func (s *SQLite) CreateEpisode(ctx context.Context, episode storage.Episode, initialState storage.EpisodeState) (int64, error) {
	if episode.State == "" {
		episode.State = storage.EpisodeStateNew
	}

	err := episode.Machine().ToState(initialState)
	if err != nil {
		return 0, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}

	setColumns := make([]sqlite.Expression, len(table.Episode.MutableColumns))
	for i, c := range table.Episode.MutableColumns {
		setColumns[i] = c
	}
	// don't insert a zeroed ID
	insertColumns := table.Episode.MutableColumns
	if episode.ID != 0 {
		insertColumns = table.Episode.AllColumns
	}

	stmt := table.Episode.
		INSERT(insertColumns).
		MODEL(episode.Episode).
		RETURNING(table.Episode.ID).
		ON_CONFLICT(table.Episode.ID).
		DO_UPDATE(sqlite.SET(table.Episode.MutableColumns.SET(sqlite.ROW(setColumns...))))

	result, err := stmt.ExecContext(ctx, tx)
	if err != nil {
		return 0, err
	}

	inserted, err := result.LastInsertId()
	if err != nil {
		tx.Rollback()
		return 0, err
	}

	state := storage.EpisodeTransition{
		EpisodeID:  int32(inserted),
		ToState:    string(initialState),
		MostRecent: true,
		SortKey:    1,
	}

	transitionStmt := table.EpisodeTransition.
		INSERT(table.EpisodeTransition.AllColumns.
			Except(table.EpisodeTransition.ID, table.EpisodeTransition.CreatedAt, table.EpisodeTransition.UpdatedAt)).
		MODEL(state)

	_, err = transitionStmt.ExecContext(ctx, tx)
	if err != nil {
		tx.Rollback()
		return 0, err
	}

	err = tx.Commit()
	if err != nil {
		tx.Rollback()
		return 0, err
	}

	return inserted, nil
}

// GetEpisode gets an episode by id
func (s *SQLite) GetEpisode(ctx context.Context, where sqlite.BoolExpression) (*storage.Episode, error) {
	stmt := sqlite.
		SELECT(
			table.Episode.AllColumns,
			table.EpisodeTransition.AllColumns,
		).
		FROM(table.Episode.
			LEFT_JOIN(table.EpisodeTransition,
				table.Episode.ID.EQ(table.EpisodeTransition.EpisodeID).
					AND(table.EpisodeTransition.MostRecent.IS_TRUE()),
			)).
		WHERE(where)

	var episode storage.Episode
	err := stmt.QueryContext(ctx, s.db, &episode)
	if err != nil {
		if errors.Is(err, qrm.ErrNoRows) {
			return nil, storage.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get episode: %w", err)
	}

	return &episode, nil
}

// DeleteEpisode removes an episode by id
func (s *SQLite) DeleteEpisode(ctx context.Context, id int64) error {
	stmt := table.Episode.
		DELETE().
		WHERE(table.Episode.ID.EQ(sqlite.Int64(id)))

	_, err := s.handleDelete(ctx, stmt)
	if err != nil {
		return fmt.Errorf("failed to delete episode: %w", err)
	}

	return nil
}

// ListEpisodes lists all episodes for a season
func (s *SQLite) ListEpisodes(ctx context.Context, where ...sqlite.BoolExpression) ([]*storage.Episode, error) {
	stmt := table.Episode.
		SELECT(
			table.Episode.AllColumns,
			table.EpisodeTransition.AllColumns,
		).
		FROM(
			table.Episode.
				INNER_JOIN(
					table.EpisodeTransition,
					table.Episode.ID.EQ(table.EpisodeTransition.EpisodeID).
						AND(table.EpisodeTransition.MostRecent.EQ(sqlite.Bool(true)))),
		)

	for _, w := range where {
		stmt = stmt.WHERE(w)
	}

	var episodes []*storage.Episode
	err := stmt.QueryContext(ctx, s.db, &episodes)
	if err != nil {
		return nil, fmt.Errorf("failed to list episodes: %w", err)
	}

	return episodes, nil
}

// GetEpisodeByEpisodeFileID gets an episode by its associated file ID
func (s *SQLite) GetEpisodeByEpisodeFileID(ctx context.Context, fileID int64) (*storage.Episode, error) {
	stmt := sqlite.SELECT(
		table.Episode.AllColumns,
		table.EpisodeTransition.ToState,
		table.EpisodeTransition.DownloadID,
		table.EpisodeTransition.DownloadClientID,
	).
		FROM(table.Episode.
			LEFT_JOIN(table.EpisodeTransition,
				table.Episode.ID.EQ(table.EpisodeTransition.EpisodeID).
					AND(table.EpisodeTransition.MostRecent.IS_TRUE()),
			)).
		WHERE(table.Episode.EpisodeFileID.EQ(sqlite.Int64(fileID)))

	var episode storage.Episode
	err := stmt.QueryContext(ctx, s.db, &episode)
	if err != nil {
		if errors.Is(err, qrm.ErrNoRows) {
			return nil, storage.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get episode by file id: %w", err)
	}

	return &episode, nil
}

// UpdateEpisodeState updates the transition state of an episode
// Metadata is optional and can be nil
func (s *SQLite) UpdateEpisodeState(ctx context.Context, id int64, state storage.EpisodeState, metadata *storage.TransitionStateMetadata) error {
	episode, err := s.GetEpisode(ctx, table.Episode.ID.EQ(sqlite.Int64(id)))
	if err != nil {
		return err
	}

	err = episode.Machine().ToState(state)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	previousTransitionStmt := table.EpisodeTransition.
		UPDATE().
		SET(
			table.EpisodeTransition.MostRecent.SET(sqlite.Bool(false))).
		WHERE(
			table.EpisodeTransition.EpisodeID.EQ(sqlite.Int(id)).
				AND(table.EpisodeTransition.MostRecent.EQ(sqlite.Bool(true)))).
		RETURNING(table.EpisodeTransition.AllColumns)

	var previousTransition storage.EpisodeTransition
	err = previousTransitionStmt.QueryContext(ctx, tx, &previousTransition)
	if err != nil {
		tx.Rollback()
		return err
	}

	transition := storage.EpisodeTransition{
		EpisodeID:  int32(id),
		ToState:    string(state),
		MostRecent: true,
		SortKey:    previousTransition.SortKey + 1,
	}

	if metadata != nil {
		if metadata.DownloadClientID != nil {
			transition.DownloadClientID = metadata.DownloadClientID
		}
		if metadata.DownloadID != nil {
			transition.DownloadID = metadata.DownloadID
		}
		if metadata.IsEntireSeasonDownload != nil {
			transition.IsEntireSeasonDownload = metadata.IsEntireSeasonDownload
		}
	}

	newTransitionStmt := table.EpisodeTransition.
		INSERT(table.EpisodeTransition.AllColumns.
			Except(table.EpisodeTransition.ID, table.EpisodeTransition.CreatedAt, table.EpisodeTransition.UpdatedAt)).
		MODEL(transition).
		RETURNING(table.EpisodeTransition.AllColumns)

	_, err = newTransitionStmt.ExecContext(ctx, tx)
	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

// UpdateEpisodeEpisodeFileID updates the episode file id for an episode
func (s *SQLite) UpdateEpisodeEpisodeFileID(ctx context.Context, id int64, fileID int64) error {
	stmt := table.Episode.
		UPDATE().
		SET(table.Episode.EpisodeFileID.SET(sqlite.Int64(fileID))).
		WHERE(table.Episode.ID.EQ(sqlite.Int64(id)))

	_, err := s.handleStatement(ctx, stmt)
	if err != nil {
		return fmt.Errorf("failed to update episode file id: %w", err)
	}

	return nil
}

// LinkEpisodeMetadata links an episode with its season and episode metadata
func (s *SQLite) LinkEpisodeMetadata(ctx context.Context, episodeID int64, seasonID int32, episodeMetadataID int32) error {
	stmt := table.Episode.UPDATE(table.Episode.SeasonID, table.Episode.EpisodeMetadataID).
		SET(seasonID, episodeMetadataID).
		WHERE(table.Episode.ID.EQ(sqlite.Int64(episodeID)))
	_, err := stmt.Exec(s.db)
	return err
}
