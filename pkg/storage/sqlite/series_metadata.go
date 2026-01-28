package sqlite

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-jet/jet/v2/qrm"
	"github.com/go-jet/jet/v2/sqlite"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/table"
)


// CreateSeriesMetadata creates the given series metadata
func (s *SQLite) CreateSeriesMetadata(ctx context.Context, seriesMeta model.SeriesMetadata) (int64, error) {
	// don't insert a zeroed ID
	insertColumns := table.SeriesMetadata.MutableColumns
	if seriesMeta.ID != 0 {
		insertColumns = table.SeriesMetadata.AllColumns
	}

	stmt := table.SeriesMetadata.
		INSERT(insertColumns).
		MODEL(seriesMeta).
		ON_CONFLICT(table.SeriesMetadata.TmdbID).
		DO_UPDATE(sqlite.SET(
			table.SeriesMetadata.Title.SET(sqlite.String(seriesMeta.Title)),
			table.SeriesMetadata.LastInfoSync.SET(sqlite.CURRENT_TIMESTAMP()),
			table.SeriesMetadata.SeasonCount.SET(sqlite.Int32(seriesMeta.SeasonCount)),
			table.SeriesMetadata.EpisodeCount.SET(sqlite.Int32(seriesMeta.EpisodeCount)),
			table.SeriesMetadata.Status.SET(sqlite.String(seriesMeta.Status)),
		))

	_, err := s.handleInsert(ctx, stmt)
	if err != nil {
		return 0, err
	}

	var existing model.SeriesMetadata
	getStmt := table.SeriesMetadata.
		SELECT(table.SeriesMetadata.ID).
		FROM(table.SeriesMetadata).
		WHERE(table.SeriesMetadata.TmdbID.EQ(sqlite.Int32(seriesMeta.TmdbID)))

	err = getStmt.QueryContext(ctx, s.db, &existing)
	if err != nil {
		return 0, fmt.Errorf("failed to get series metadata ID after upsert: %w", err)
	}

	return int64(existing.ID), nil
}

// UpdateSeriesMetadata updates an existing series metadata record
func (s *SQLite) UpdateSeriesMetadata(ctx context.Context, metadata model.SeriesMetadata) error {
	now := time.Now()
	metadata.LastInfoSync = &now

	stmt := table.SeriesMetadata.
		UPDATE(table.SeriesMetadata.AllColumns.
			Except(
				table.SeriesMetadata.ID,
				table.SeriesMetadata.TmdbID,
			)).
		MODEL(metadata).
		WHERE(table.SeriesMetadata.ID.EQ(sqlite.Int32(metadata.ID)))

	_, err := s.handleStatement(ctx, stmt)
	if err != nil {
		return fmt.Errorf("failed to update series metadata: %w", err)
	}

	return nil
}

// DeleteSeriesMetadata deletes a Series metadata by id
func (s *SQLite) DeleteSeriesMetadata(ctx context.Context, id int64) error {
	stmt := table.SeriesMetadata.
		DELETE().
		WHERE(table.SeriesMetadata.ID.EQ(sqlite.Int64(id)))

	_, err := s.handleDelete(ctx, stmt)
	if err != nil {
		return fmt.Errorf("failed to delete Series metadata: %w", err)
	}

	return nil
}

// ListSeriesMetadata lists all Series metadata
func (s *SQLite) ListSeriesMetadata(ctx context.Context, where ...sqlite.BoolExpression) ([]*model.SeriesMetadata, error) {
	stmt := table.SeriesMetadata.
		SELECT(table.SeriesMetadata.AllColumns).
		FROM(table.SeriesMetadata)

	for _, w := range where {
		stmt = stmt.WHERE(w)
	}

	var SeriesMetadata []*model.SeriesMetadata
	err := stmt.QueryContext(ctx, s.db, &SeriesMetadata)
	if err != nil {
		return nil, fmt.Errorf("failed to list Series metadata: %w", err)
	}

	return SeriesMetadata, nil
}

// GetSeriesMetadata get a Series metadata for the given where
func (s *SQLite) GetSeriesMetadata(ctx context.Context, where sqlite.BoolExpression) (*model.SeriesMetadata, error) {
	stmt := table.SeriesMetadata.
		SELECT(table.SeriesMetadata.AllColumns).
		FROM(table.SeriesMetadata).
		WHERE(where)

	var seriesMetadata model.SeriesMetadata
	err := stmt.QueryContext(ctx, s.db, &seriesMetadata)
	if err != nil {
		if errors.Is(err, qrm.ErrNoRows) {
			return nil, storage.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get Series metadata: %w", err)
	}

	return &seriesMetadata, nil
}

// UpdateSeriesState updates the transition state of a series
// Metadata is optional and can be nil
func (s *SQLite) UpdateSeriesState(ctx context.Context, id int64, state storage.SeriesState, metadata *storage.TransitionStateMetadata) error {
	series, err := s.GetSeries(ctx, table.Series.ID.EQ(sqlite.Int64(id)))
	if err != nil {
		return err
	}

	err = series.Machine().ToState(state)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	previousTransitionStmt := table.SeriesTransition.
		UPDATE().
		SET(
			table.SeriesTransition.MostRecent.SET(sqlite.Bool(false))).
		WHERE(
			table.SeriesTransition.SeriesID.EQ(sqlite.Int(id)).
				AND(table.SeriesTransition.MostRecent.EQ(sqlite.Bool(true)))).
		RETURNING(table.SeriesTransition.AllColumns)

	var previousTransition storage.SeriesTransition
	err = previousTransitionStmt.QueryContext(ctx, tx, &previousTransition)
	if err != nil {
		tx.Rollback()
		return err
	}

	transition := storage.SeriesTransition{
		SeriesID:   int32(id),
		ToState:    string(state),
		MostRecent: true,
		SortKey:    previousTransition.SortKey + 1,
	}

	// Note: SeriesTransition doesn't currently support DownloadClientID and DownloadID
	// These can be added to the schema if needed in the future

	newTransitionStmt := table.SeriesTransition.
		INSERT(table.SeriesTransition.AllColumns.
			Except(table.SeriesTransition.ID, table.SeriesTransition.CreatedAt, table.SeriesTransition.UpdatedAt)).
		MODEL(transition).
		RETURNING(table.SeriesTransition.AllColumns)

	_, err = newTransitionStmt.ExecContext(ctx, tx)
	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

// UpdateSeries updates fields on a series
func (s *SQLite) UpdateSeries(ctx context.Context, series model.Series, where ...sqlite.BoolExpression) error {
	stmt := table.Series.UPDATE(table.Series.Monitored).MODEL(series)
	for _, w := range where {
		stmt = stmt.WHERE(w)
	}
	_, err := stmt.ExecContext(ctx, s.db)
	return err
}

// LinkSeriesMetadata links a series with its metadata
func (s *SQLite) LinkSeriesMetadata(ctx context.Context, seriesID int64, metadataID int32) error {
	stmt := table.Series.UPDATE(table.Series.SeriesMetadataID).SET(metadataID).WHERE(table.Series.ID.EQ(sqlite.Int64(seriesID)))
	_, err := stmt.Exec(s.db)
	return err
}
