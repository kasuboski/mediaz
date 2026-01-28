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

// CreateSeries stores a Series in the database
func (s *SQLite) CreateSeries(ctx context.Context, series storage.Series, initialState storage.SeriesState) (int64, error) {
	if series.State == "" {
		series.State = storage.SeriesStateNew
	}

	err := series.Machine().ToState(initialState)
	if err != nil {
		return 0, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}

	setColumns := make([]sqlite.Expression, len(table.Series.MutableColumns))
	for i, c := range table.Series.MutableColumns {
		setColumns[i] = c
	}

	insertColumns := table.Series.MutableColumns
	if series.ID == 0 {
		insertColumns = insertColumns.Except(table.Series.ID)
	}
	if series.Added == nil || series.Added.IsZero() {
		insertColumns = insertColumns.Except(table.Series.Added)
	}

	stmt := table.Series.
		INSERT(insertColumns).
		MODEL(series.Series).
		RETURNING(table.Series.ID).
		ON_CONFLICT(table.Series.ID).
		DO_UPDATE(sqlite.SET(table.Series.MutableColumns.SET(sqlite.ROW(setColumns...))))

	result, err := stmt.ExecContext(ctx, tx)
	if err != nil {
		tx.Rollback()
		return 0, err
	}

	inserted, err := result.LastInsertId()
	if err != nil {
		tx.Rollback()
		return 0, err
	}

	state := storage.SeriesTransition{
		SeriesID:   int32(inserted),
		ToState:    string(initialState),
		MostRecent: true,
		SortKey:    1,
	}

	transitionStmt := table.SeriesTransition.
		INSERT(table.SeriesTransition.AllColumns.
			Except(table.SeriesTransition.ID, table.SeriesTransition.CreatedAt, table.SeriesTransition.UpdatedAt)).
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

// GetSeries looks for a series given a where condition
func (s *SQLite) GetSeries(ctx context.Context, where sqlite.BoolExpression) (*storage.Series, error) {
	stmt := table.Series.
		SELECT(
			table.Series.AllColumns,
			table.SeriesTransition.AllColumns,
		).
		FROM(
			table.Series.
				INNER_JOIN(
					table.SeriesTransition,
					table.Series.ID.EQ(table.SeriesTransition.SeriesID).
						AND(table.SeriesTransition.MostRecent.EQ(sqlite.Bool(true)))),
		).
		WHERE(
			where,
		)

	var series storage.Series
	err := stmt.QueryContext(ctx, s.db, &series)
	if err != nil {
		if errors.Is(err, qrm.ErrNoRows) {
			return nil, storage.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get series: %w", err)
	}

	return &series, nil
}

// DeleteSeries removes a Series by id
func (s *SQLite) DeleteSeries(ctx context.Context, id int64) error {
	stmt := table.Series.
		DELETE().
		WHERE(table.Series.ID.EQ(sqlite.Int64(id)))

	_, err := s.handleDelete(ctx, stmt)
	if err != nil {
		return fmt.Errorf("failed to delete Series: %w", err)
	}

	return nil
}

// ListSeries lists all Series
func (s *SQLite) ListSeries(ctx context.Context, where ...sqlite.BoolExpression) ([]*storage.Series, error) {
	stmt := table.Series.
		SELECT(
			table.Series.AllColumns,
			table.SeriesTransition.AllColumns,
		).
		FROM(
			table.Series.
				INNER_JOIN(
					table.SeriesTransition,
					table.Series.ID.EQ(table.SeriesTransition.SeriesID).
						AND(table.SeriesTransition.MostRecent.EQ(sqlite.Bool(true)))).
				LEFT_JOIN(
					table.SeriesMetadata,
					table.Series.SeriesMetadataID.EQ(table.SeriesMetadata.ID)),
		).
		ORDER_BY(table.SeriesMetadata.Title.ASC())

	for _, w := range where {
		stmt = stmt.WHERE(w)
	}

	var series []*storage.Series
	err := stmt.QueryContext(ctx, s.db, &series)
	if err != nil {
		return nil, fmt.Errorf("failed to list series: %w", err)
	}

	return series, nil
}
