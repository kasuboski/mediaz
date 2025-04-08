package sqlite

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-jet/jet/v2/qrm"
	"github.com/go-jet/jet/v2/sqlite"
	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/table"
)

// CreateShow stores a show in the database
func (s SQLite) CreateShow(ctx context.Context, show model.Series) (int64, error) {
	setColumns := make([]sqlite.Expression, len(table.Series.MutableColumns))
	for i, c := range table.Series.MutableColumns {
		setColumns[i] = c
	}
	// don't insert a zeroed ID
	insertColumns := table.Series.MutableColumns
	if show.ID != 0 {
		insertColumns = table.Series.AllColumns
	}

	stmt := table.Series.
		INSERT(insertColumns).
		MODEL(show).
		RETURNING(table.Series.ID).
		ON_CONFLICT(table.Series.ID).
		DO_UPDATE(sqlite.SET(table.Series.MutableColumns.SET(sqlite.ROW(setColumns...))))

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

// GetShow gets a show by id
func (s SQLite) GetShow(ctx context.Context, id int64) (*model.Series, error) {
	stmt := table.Series.
		SELECT(table.Series.AllColumns).
		WHERE(table.Series.ID.EQ(sqlite.Int64(id)))

	var show model.Series
	err := stmt.QueryContext(ctx, s.db, &show)
	if err != nil {
		if errors.Is(err, qrm.ErrNoRows) {
			return nil, storage.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get show: %w", err)
	}

	return &show, nil
}

// DeleteShow removes a show by id
func (s SQLite) DeleteShow(ctx context.Context, id int64) error {
	stmt := table.Series.
		DELETE().
		WHERE(table.Series.ID.EQ(sqlite.Int64(id)))

	_, err := s.handleDelete(ctx, stmt)
	if err != nil {
		return fmt.Errorf("failed to delete show: %w", err)
	}

	return nil
}

// ListShows lists all shows
func (s SQLite) ListShows(ctx context.Context) ([]*model.Series, error) {
	stmt := table.Series.
		SELECT(table.Series.AllColumns)

	var shows []*model.Series
	err := stmt.QueryContext(ctx, s.db, &shows)
	if err != nil {
		return nil, fmt.Errorf("failed to list shows: %w", err)
	}

	return shows, nil
}

// CreateSeason stores a season in the database
func (s SQLite) CreateSeason(ctx context.Context, season model.Season) (int64, error) {
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
		MODEL(season).
		RETURNING(table.Season.ID).
		ON_CONFLICT(table.Season.ID).
		DO_UPDATE(sqlite.SET(table.Season.MutableColumns.SET(sqlite.ROW(setColumns...))))

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

// GetSeason gets a season by id
func (s SQLite) GetSeason(ctx context.Context, id int64) (*model.Season, error) {
	stmt := table.Season.
		SELECT(table.Season.AllColumns).
		WHERE(table.Season.ID.EQ(sqlite.Int64(id)))

	var season model.Season
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

// ListSeasons lists all seasons for a show
func (s SQLite) ListSeasons(ctx context.Context, showID int64) ([]*model.Season, error) {
	stmt := table.Season.
		SELECT(table.Season.AllColumns).
		WHERE(table.Season.SeriesID.EQ(sqlite.Int64(showID)))

	var seasons []*model.Season
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
	defer tx.Rollback()

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

	// Create initial transition state
	state := model.EpisodeTransition{
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
func (s SQLite) GetEpisode(ctx context.Context, id int64) (*storage.Episode, error) {
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
		WHERE(table.Episode.ID.EQ(sqlite.Int64(id)))

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
func (s SQLite) ListEpisodes(ctx context.Context, seasonID int64) ([]*storage.Episode, error) {
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
		WHERE(table.Episode.SeasonID.EQ(sqlite.Int64(seasonID)))

	var episodes []*storage.Episode
	err := stmt.QueryContext(ctx, s.db, &episodes)
	if err != nil {
		return nil, fmt.Errorf("failed to list episodes: %w", err)
	}

	return episodes, nil
}

// ListEpisodesByState lists all episodes in a given state
func (s SQLite) ListEpisodesByState(ctx context.Context, state storage.EpisodeState) ([]*storage.Episode, error) {
	stmt := sqlite.SELECT(
		table.Episode.AllColumns,
		table.EpisodeTransition.ToState,
		table.EpisodeTransition.DownloadID,
		table.EpisodeTransition.DownloadClientID,
	).
		FROM(table.Episode.
			INNER_JOIN(table.EpisodeTransition,
				table.Episode.ID.EQ(table.EpisodeTransition.EpisodeID).
					AND(table.EpisodeTransition.MostRecent.IS_TRUE()),
			)).
		WHERE(table.EpisodeTransition.ToState.EQ(sqlite.String(string(state))))

	var episodes []*storage.Episode
	err := stmt.QueryContext(ctx, s.db, &episodes)
	if err != nil {
		return nil, fmt.Errorf("failed to list episodes by state: %w", err)
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

// CreateShowMetadata creates the given showMeta
func (s SQLite) CreateShowMetadata(ctx context.Context, showMeta model.SeriesMetadata) (int64, error) {
	setColumns := make([]sqlite.Expression, len(table.SeriesMetadata.MutableColumns))
	for i, c := range table.SeriesMetadata.MutableColumns {
		setColumns[i] = c
	}
	// don't insert a zeroed ID
	insertColumns := table.SeriesMetadata.MutableColumns
	if showMeta.ID != 0 {
		insertColumns = table.SeriesMetadata.AllColumns
	}

	stmt := table.SeriesMetadata.
		INSERT(insertColumns).
		MODEL(showMeta).
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

// DeleteShowMetadata deletes a show metadata by id
func (s SQLite) DeleteShowMetadata(ctx context.Context, id int64) error {
	stmt := table.SeriesMetadata.
		DELETE().
		WHERE(table.SeriesMetadata.ID.EQ(sqlite.Int64(id)))

	_, err := s.handleDelete(ctx, stmt)
	if err != nil {
		return fmt.Errorf("failed to delete show metadata: %w", err)
	}

	return nil
}

// ListShowMetadata lists all show metadata
func (s SQLite) ListShowMetadata(ctx context.Context) ([]*model.SeriesMetadata, error) {
	stmt := table.SeriesMetadata.
		SELECT(table.SeriesMetadata.AllColumns).
		FROM(table.SeriesMetadata)

	var showMetadata []*model.SeriesMetadata
	err := stmt.QueryContext(ctx, s.db, &showMetadata)
	if err != nil {
		return nil, fmt.Errorf("failed to list show metadata: %w", err)
	}

	return showMetadata, nil
}

// GetShowMetadata get a show metadata for the given where
func (s SQLite) GetShowMetadata(ctx context.Context, where sqlite.BoolExpression) (*model.SeriesMetadata, error) {
	stmt := table.SeriesMetadata.
		SELECT(table.SeriesMetadata.AllColumns).
		FROM(table.SeriesMetadata).
		WHERE(where)

	var showMetadata model.SeriesMetadata
	err := stmt.QueryContext(ctx, s.db, &showMetadata)
	if err != nil {
		if errors.Is(err, qrm.ErrNoRows) {
			return nil, storage.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get show metadata: %w", err)
	}

	return &showMetadata, nil
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
func (s SQLite) ListSeasonMetadata(ctx context.Context) ([]*model.SeasonMetadata, error) {
	stmt := table.SeasonMetadata.
		SELECT(table.SeasonMetadata.AllColumns).
		FROM(table.SeasonMetadata)

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
func (s SQLite) ListEpisodeMetadata(ctx context.Context) ([]*model.EpisodeMetadata, error) {
	stmt := table.EpisodeMetadata.
		SELECT(table.EpisodeMetadata.AllColumns).
		FROM(table.EpisodeMetadata)

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
