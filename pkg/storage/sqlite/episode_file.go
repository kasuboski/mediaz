package sqlite

import (
	"context"
	"fmt"

	"github.com/go-jet/jet/v2/sqlite"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/table"
)

// GetEpisodeFileByID gets all episode files for an episode
func (s *SQLite) GetEpisodeFileByID(ctx context.Context, id int64) ([]*model.EpisodeFile, error) {
	stmt := table.EpisodeFile.
		SELECT(table.EpisodeFile.AllColumns).
		FROM(table.EpisodeFile).
		WHERE(table.EpisodeFile.ID.EQ(sqlite.Int64(id)))

	var result []*model.EpisodeFile
	err := stmt.QueryContext(ctx, s.db, &result)
	if err != nil {
		return result, err
	}

	if len(result) == 0 {
		return nil, storage.ErrNotFound
	}

	return result, err
}

func (s *SQLite) GetEpisodeFile(ctx context.Context, id int32) (*model.EpisodeFile, error) {
	stmt := table.EpisodeFile.
		SELECT(table.EpisodeFile.AllColumns).
		FROM(table.EpisodeFile).
		WHERE(table.EpisodeFile.ID.EQ(sqlite.Int32(id)))

	var result model.EpisodeFile
	err := stmt.QueryContext(ctx, s.db, &result)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func (s *SQLite) UpdateEpisodeFile(ctx context.Context, id int32, file model.EpisodeFile) error {
	stmt := table.EpisodeFile.
		UPDATE(table.EpisodeFile.MutableColumns).
		MODEL(file).
		WHERE(table.EpisodeFile.ID.EQ(sqlite.Int32(id)))

	_, err := s.handleStatement(ctx, stmt)
	return err
}

// CreateEpisodeFile stores an episode file
func (s *SQLite) CreateEpisodeFile(ctx context.Context, file model.EpisodeFile) (int64, error) {
	setColumns := make([]sqlite.Expression, len(table.EpisodeFile.MutableColumns))
	for i, c := range table.EpisodeFile.MutableColumns {
		setColumns[i] = c
	}
	insertColumns := table.EpisodeFile.MutableColumns.Except(table.EpisodeFile.Added)
	if file.ID != 0 {
		insertColumns = table.EpisodeFile.AllColumns
	}

	stmt := table.EpisodeFile.
		INSERT(insertColumns).
		MODEL(file).
		ON_CONFLICT(table.EpisodeFile.ID).
		DO_UPDATE(sqlite.SET(table.EpisodeFile.MutableColumns.SET(sqlite.ROW(setColumns...)))).
		RETURNING(table.EpisodeFile.ID)

	result, err := s.handleInsert(ctx, stmt)
	if err != nil {
		return 0, err
	}

	inserted, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	return inserted, nil
}

// DeleteEpisodeFile removes an episode file by id
func (s *SQLite) DeleteEpisodeFile(ctx context.Context, id int64) error {
	stmt := table.EpisodeFile.
		DELETE().
		WHERE(table.EpisodeFile.ID.EQ(sqlite.Int64(id))).
		RETURNING(table.EpisodeFile.ID)

	_, err := s.handleDelete(ctx, stmt)
	if err != nil {
		return err
	}
	return nil
}

// ListEpisodeFiles lists all episode files
func (s *SQLite) ListEpisodeFiles(ctx context.Context) ([]*model.EpisodeFile, error) {
	episodeFiles := make([]*model.EpisodeFile, 0)
	stmt := table.EpisodeFile.
		SELECT(table.EpisodeFile.AllColumns).
		FROM(table.EpisodeFile).
		ORDER_BY(table.EpisodeFile.ID.ASC())

	err := stmt.QueryContext(ctx, s.db, &episodeFiles)
	if err != nil {
		return nil, fmt.Errorf("failed to list episode files: %w", err)
	}

	return episodeFiles, nil
}
