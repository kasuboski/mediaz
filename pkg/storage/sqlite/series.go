package sqlite

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-jet/jet/v2/qrm"
	"github.com/go-jet/jet/v2/sqlite"
	"github.com/kasuboski/mediaz/pkg/logger"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/table"
)

// CreateSeries stores a Series in the database
func (s SQLite) CreateSeries(ctx context.Context, series storage.Series, initialState storage.SeriesState) (int64, error) {
	if series.State == "" {
		series.State = storage.SeriesStateNew
	}

	err := series.Machine().ToState(initialState)
	if err != nil {
		return 0, err
	}

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
func (s SQLite) GetSeries(ctx context.Context, where sqlite.BoolExpression) (*storage.Series, error) {
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
func (s SQLite) DeleteSeries(ctx context.Context, id int64) error {
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
func (s SQLite) ListSeries(ctx context.Context, where ...sqlite.BoolExpression) ([]*storage.Series, error) {
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
		)

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

// CreateSeason stores a season in the database
func (s SQLite) CreateSeason(ctx context.Context, season storage.Season, initialState storage.SeasonState) (int64, error) {
	if season.State == "" {
		season.State = storage.SeasonStateNew
	}

	err := season.Machine().ToState(initialState)
	if err != nil {
		return 0, err
	}

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
func (s SQLite) GetSeason(ctx context.Context, where sqlite.BoolExpression) (*storage.Season, error) {
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
func (s SQLite) DeleteSeason(ctx context.Context, id int64) error {
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
func (s SQLite) ListSeasons(ctx context.Context, where ...sqlite.BoolExpression) ([]*storage.Season, error) {
	log := logger.FromCtx(ctx)
	stmt := table.Series.
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

	log.Debug(stmt.DebugSql())

	var seasons []*storage.Season
	err := stmt.QueryContext(ctx, s.db, &seasons)
	if err != nil {
		return nil, fmt.Errorf("failed to list seasons: %w", err)
	}

	return seasons, nil
}

// CreateEpisode stores an episode and creates an initial transition state
func (s SQLite) CreateEpisode(ctx context.Context, episode storage.Episode, initialState storage.EpisodeState) (int64, error) {
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
func (s SQLite) GetEpisode(ctx context.Context, where sqlite.BoolExpression) (*storage.Episode, error) {
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
func (s SQLite) DeleteEpisode(ctx context.Context, id int64) error {
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
func (s SQLite) ListEpisodes(ctx context.Context, where ...sqlite.BoolExpression) ([]*storage.Episode, error) {
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
func (s SQLite) GetEpisodeByEpisodeFileID(ctx context.Context, fileID int64) (*storage.Episode, error) {
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
func (s SQLite) UpdateEpisodeState(ctx context.Context, id int64, state storage.EpisodeState, metadata *storage.TransitionStateMetadata) error {
	episode, err := s.GetEpisode(ctx, table.Episode.ID.EQ(sqlite.Int64(id)))
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
		if metadata.DownloadClientID != nil && metadata.DownloadID != nil {
			transition.DownloadClientID = metadata.DownloadClientID
			transition.DownloadID = metadata.DownloadID
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
func (s SQLite) UpdateEpisodeEpisodeFileID(ctx context.Context, id int64, fileID int64) error {
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

// GetEpisodeFiles gets all episode files for an episode
func (s SQLite) GetEpisodeFiles(ctx context.Context, id int64) ([]*model.EpisodeFile, error) {
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

// CreateEpisodeFile stores an episode file
func (s SQLite) CreateEpisodeFile(ctx context.Context, file model.EpisodeFile) (int64, error) {
	setColumns := make([]sqlite.Expression, len(table.EpisodeFile.MutableColumns))
	for i, c := range table.EpisodeFile.MutableColumns {
		setColumns[i] = c
	}
	// don't insert a zeroed ID
	insertColumns := table.EpisodeFile.MutableColumns
	if file.ID != 0 {
		insertColumns = table.EpisodeFile.AllColumns
	}

	stmt := table.EpisodeFile.
		INSERT(insertColumns).
		MODEL(file).
		RETURNING(table.EpisodeFile.ID).
		ON_CONFLICT(table.EpisodeFile.ID).
		DO_UPDATE(sqlite.SET(table.EpisodeFile.MutableColumns.SET(sqlite.ROW(setColumns...))))

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
func (s SQLite) DeleteEpisodeFile(ctx context.Context, id int64) error {
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
func (s SQLite) ListEpisodeFiles(ctx context.Context) ([]*model.EpisodeFile, error) {
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

// CreateSeriesMetadata creates the given series metadata
func (s SQLite) CreateSeriesMetadata(ctx context.Context, seriesMeta model.SeriesMetadata) (int64, error) {
	setColumns := make([]sqlite.Expression, len(table.SeriesMetadata.MutableColumns))
	for i, c := range table.SeriesMetadata.MutableColumns {
		setColumns[i] = c
	}
	// don't insert a zeroed ID
	insertColumns := table.SeriesMetadata.MutableColumns
	if seriesMeta.ID != 0 {
		insertColumns = table.SeriesMetadata.AllColumns
	}

	stmt := table.SeriesMetadata.
		INSERT(insertColumns).
		MODEL(seriesMeta).
		RETURNING(table.SeriesMetadata.ID).
		ON_CONFLICT(table.SeriesMetadata.ID).
		DO_UPDATE(sqlite.SET(table.SeriesMetadata.MutableColumns.SET(sqlite.ROW(setColumns...))))

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

// DeleteSeriesMetadata deletes a Series metadata by id
func (s SQLite) DeleteSeriesMetadata(ctx context.Context, id int64) error {
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
func (s SQLite) ListSeriesMetadata(ctx context.Context, where ...sqlite.BoolExpression) ([]*model.SeriesMetadata, error) {
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
func (s SQLite) GetSeriesMetadata(ctx context.Context, where sqlite.BoolExpression) (*model.SeriesMetadata, error) {
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

// CreateSeasonMetadata creates the given seasonMeta
func (s SQLite) CreateSeasonMetadata(ctx context.Context, seasonMeta model.SeasonMetadata) (int64, error) {
	setColumns := make([]sqlite.Expression, len(table.SeasonMetadata.MutableColumns))
	for i, c := range table.SeasonMetadata.MutableColumns {
		setColumns[i] = c
	}
	// don't insert a zeroed ID
	insertColumns := table.SeasonMetadata.MutableColumns
	if seasonMeta.ID != 0 {
		insertColumns = table.SeasonMetadata.AllColumns
	}

	stmt := table.SeasonMetadata.
		INSERT(insertColumns).
		MODEL(seasonMeta).
		RETURNING(table.SeasonMetadata.ID).
		ON_CONFLICT(table.SeasonMetadata.ID).
		DO_UPDATE(sqlite.SET(table.SeasonMetadata.MutableColumns.SET(sqlite.ROW(setColumns...))))

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

// DeleteSeasonMetadata deletes a season metadata by id
func (s SQLite) DeleteSeasonMetadata(ctx context.Context, id int64) error {
	stmt := table.SeasonMetadata.
		DELETE().
		WHERE(table.SeasonMetadata.ID.EQ(sqlite.Int64(id)))

	_, err := s.handleDelete(ctx, stmt)
	if err != nil {
		return fmt.Errorf("failed to delete season metadata: %w", err)
	}

	return nil
}

// ListSeasonMetadata lists all season metadata
func (s SQLite) ListSeasonMetadata(ctx context.Context, where ...sqlite.BoolExpression) ([]*model.SeasonMetadata, error) {
	stmt := table.SeasonMetadata.
		SELECT(table.SeasonMetadata.AllColumns).
		FROM(table.SeasonMetadata)

	for _, w := range where {
		stmt = stmt.WHERE(w)
	}

	var seasonMetadata []*model.SeasonMetadata
	err := stmt.QueryContext(ctx, s.db, &seasonMetadata)
	if err != nil {
		return nil, fmt.Errorf("failed to list season metadata: %w", err)
	}

	return seasonMetadata, nil
}

// GetSeasonMetadata get a season metadata for the given where
func (s SQLite) GetSeasonMetadata(ctx context.Context, where sqlite.BoolExpression) (*model.SeasonMetadata, error) {
	stmt := table.SeasonMetadata.
		SELECT(table.SeasonMetadata.AllColumns).
		FROM(table.SeasonMetadata).
		WHERE(where)

	var seasonMetadata model.SeasonMetadata
	err := stmt.QueryContext(ctx, s.db, &seasonMetadata)
	if err != nil {
		if errors.Is(err, qrm.ErrNoRows) {
			return nil, storage.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get season metadata: %w", err)
	}

	return &seasonMetadata, nil
}

// CreateEpisodeMetadata creates the given episodeMeta
func (s SQLite) CreateEpisodeMetadata(ctx context.Context, episodeMeta model.EpisodeMetadata) (int64, error) {
	setColumns := make([]sqlite.Expression, len(table.EpisodeMetadata.MutableColumns))
	for i, c := range table.EpisodeMetadata.MutableColumns {
		setColumns[i] = c
	}
	// don't insert a zeroed ID
	insertColumns := table.EpisodeMetadata.MutableColumns
	if episodeMeta.ID != 0 {
		insertColumns = table.EpisodeMetadata.AllColumns
	}

	stmt := table.EpisodeMetadata.
		INSERT(insertColumns).
		MODEL(episodeMeta).
		RETURNING(table.EpisodeMetadata.ID).
		ON_CONFLICT(table.EpisodeMetadata.ID).
		DO_UPDATE(sqlite.SET(table.EpisodeMetadata.MutableColumns.SET(sqlite.ROW(setColumns...))))

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

// DeleteEpisodeMetadata deletes an episode metadata by id
func (s SQLite) DeleteEpisodeMetadata(ctx context.Context, id int64) error {
	stmt := table.EpisodeMetadata.
		DELETE().
		WHERE(table.EpisodeMetadata.ID.EQ(sqlite.Int64(id)))

	_, err := s.handleDelete(ctx, stmt)
	if err != nil {
		return fmt.Errorf("failed to delete episode metadata: %w", err)
	}

	return nil
}

// ListEpisodeMetadata lists all episode metadata
func (s SQLite) ListEpisodeMetadata(ctx context.Context, where ...sqlite.BoolExpression) ([]*model.EpisodeMetadata, error) {
	stmt := table.EpisodeMetadata.
		SELECT(table.EpisodeMetadata.AllColumns).
		FROM(table.EpisodeMetadata)

	for _, w := range where {
		stmt = stmt.WHERE(w)
	}

	var episodeMetadata []*model.EpisodeMetadata
	err := stmt.QueryContext(ctx, s.db, &episodeMetadata)
	if err != nil {
		return nil, fmt.Errorf("failed to list episode metadata: %w", err)
	}

	return episodeMetadata, nil
}

// GetEpisodeMetadata get an episode metadata for the given where
func (s SQLite) GetEpisodeMetadata(ctx context.Context, where sqlite.BoolExpression) (*model.EpisodeMetadata, error) {
	stmt := table.EpisodeMetadata.
		SELECT(table.EpisodeMetadata.AllColumns).
		FROM(table.EpisodeMetadata).
		WHERE(where)

	var episodeMetadata model.EpisodeMetadata
	err := stmt.QueryContext(ctx, s.db, &episodeMetadata)
	if err != nil {
		if errors.Is(err, qrm.ErrNoRows) {
			return nil, storage.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get episode metadata: %w", err)
	}

	return &episodeMetadata, nil
}
