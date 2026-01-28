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

// LinkSeasonMetadata links a season with its metadata
func (s *SQLite) LinkSeasonMetadata(ctx context.Context, seasonID int64, metadataID int32) error {
	stmt := table.Season.UPDATE(table.Season.SeasonMetadataID).SET(metadataID).WHERE(table.Season.ID.EQ(sqlite.Int64(seasonID)))
	_, err := stmt.Exec(s.db)
	return err
}

// CreateSeason stores a season in the database
func (s *SQLite) CreateSeason(ctx context.Context, season storage.Season, initialState storage.SeasonState) (int64, error) {
	if season.State == "" {
		season.State = storage.SeasonStateNew
	}

	err := season.Machine().ToState(initialState)
	if err != nil {
		return 0, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}

	setColumns := make([]sqlite.Expression, len(table.Season.MutableColumns))
	for i, c := range table.Season.MutableColumns {
		setColumns[i] = c
	}
	// don't insert a zeroed ID
	insertColumns := table.Season.MutableColumns
	if season.ID != 0 {
		insertColumns = table.Season.AllColumns
	}

	stmt := table.Season.
		INSERT(insertColumns).
		MODEL(season.Season).
		RETURNING(table.Season.ID).
		ON_CONFLICT(table.Season.ID).
		DO_UPDATE(sqlite.SET(table.Season.MutableColumns.SET(sqlite.ROW(setColumns...))))

	result, err := stmt.ExecContext(ctx, tx)
	if err != nil {
		return 0, err
	}

	inserted, err := result.LastInsertId()
	if err != nil {
		tx.Rollback()
		return 0, err
	}

	state := storage.SeasonTransition{
		SeasonID:   int32(inserted),
		ToState:    string(initialState),
		MostRecent: true,
		SortKey:    1,
	}

	transitionStmt := table.SeasonTransition.
		INSERT(table.SeasonTransition.AllColumns.
			Except(table.SeasonTransition.ID, table.SeasonTransition.CreatedAt, table.SeasonTransition.UpdatedAt)).
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

// GetSeason gets a season by id
func (s *SQLite) GetSeason(ctx context.Context, where sqlite.BoolExpression) (*storage.Season, error) {
	stmt := sqlite.
		SELECT(
			table.Season.AllColumns,
			table.SeasonTransition.AllColumns,
		).
		FROM(table.Season.
			LEFT_JOIN(table.SeasonTransition,
				table.Season.ID.EQ(table.SeasonTransition.SeasonID).
					AND(table.SeasonTransition.MostRecent.IS_TRUE()),
			)).
		WHERE(where)

	var season storage.Season
	err := stmt.QueryContext(ctx, s.db, &season)
	if err != nil {
		if errors.Is(err, qrm.ErrNoRows) {
			return nil, storage.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get season: %w", err)
	}

	return &season, nil
}

// DeleteSeason removes a season by id
func (s *SQLite) DeleteSeason(ctx context.Context, id int64) error {
	stmt := table.Season.
		DELETE().
		WHERE(table.Season.ID.EQ(sqlite.Int64(id)))

	_, err := s.handleDelete(ctx, stmt)
	if err != nil {
		return fmt.Errorf("failed to delete season: %w", err)
	}

	return nil
}

// ListSeasons lists all seasons for a Series
func (s *SQLite) ListSeasons(ctx context.Context, where ...sqlite.BoolExpression) ([]*storage.Season, error) {
	stmt := table.Season.
		SELECT(
			table.Season.AllColumns,
			table.SeasonTransition.AllColumns,
		).
		FROM(
			table.Season.
				INNER_JOIN(
					table.SeasonTransition,
					table.Season.ID.EQ(table.SeasonTransition.SeasonID).
						AND(table.SeasonTransition.MostRecent.EQ(sqlite.Bool(true)))),
		)

	for _, w := range where {
		stmt = stmt.WHERE(w)
	}
	var seasons []*storage.Season
	err := stmt.QueryContext(ctx, s.db, &seasons)
	if err != nil {
		return nil, fmt.Errorf("failed to list seasons: %w", err)
	}

	return seasons, nil
}

// UpdateSeasonState updates the transition state of an episode
// Metadata is optional and can be nil
func (s *SQLite) UpdateSeasonState(ctx context.Context, id int64, state storage.SeasonState, metadata *storage.TransitionStateMetadata) error {
	season, err := s.GetSeason(ctx, table.Season.ID.EQ(sqlite.Int64(id)))
	if err != nil {
		return err
	}

	err = season.Machine().ToState(state)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	previousTransitionStmt := table.SeasonTransition.
		UPDATE().
		SET(
			table.SeasonTransition.MostRecent.SET(sqlite.Bool(false))).
		WHERE(
			table.SeasonTransition.SeasonID.EQ(sqlite.Int(id)).
				AND(table.SeasonTransition.MostRecent.EQ(sqlite.Bool(true)))).
		RETURNING(table.SeasonTransition.AllColumns)

	var previousTransition storage.SeasonTransition
	err = previousTransitionStmt.QueryContext(ctx, tx, &previousTransition)
	if err != nil {
		tx.Rollback()
		return err
	}

	transition := storage.SeasonTransition{
		SeasonID:   int32(id),
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

	newTransitionStmt := table.SeasonTransition.
		INSERT(table.SeasonTransition.AllColumns.
			Except(table.SeasonTransition.ID, table.SeasonTransition.CreatedAt, table.SeasonTransition.UpdatedAt)).
		MODEL(transition).
		RETURNING(table.SeasonTransition.AllColumns)

	_, err = newTransitionStmt.ExecContext(ctx, tx)
	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}
