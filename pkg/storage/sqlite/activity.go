package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/kasuboski/mediaz/pkg/storage"
)

type movieTimelineEntry struct {
	Date        string
	Downloaded  int
	Downloading int
}

type seriesTimelineEntry struct {
	Date        string
	Completed   int
	Downloading int
}

type jobTimelineEntry struct {
	Date  string
	Done  int
	Error int
}

func (s *SQLite) ListDownloadingMovies(ctx context.Context) ([]*storage.ActiveMovie, error) {
	query := `
		SELECT
			m.id,
			mm.tmdb_id,
			mm.title,
			mm.year,
			mm.images,
			mt.to_state,
			mt.sort_key,
			mt.download_id,
			dc.id,
			dc.host,
			dc.port
		FROM movie m
		INNER JOIN movie_transition mt ON m.id = mt.movie_id AND mt.most_recent = 1
		INNER JOIN movie_metadata mm ON m.movie_metadata_id = mm.id
		LEFT JOIN download_client dc ON mt.download_client_id = dc.id
		WHERE mt.to_state = 'downloading'
		ORDER BY mt.sort_key DESC
	`

	s.mu.Lock()
	rows, err := s.db.QueryContext(ctx, query)
	s.mu.Unlock()

	if err != nil {
		return nil, fmt.Errorf("failed to query downloading movies: %w", err)
	}
	defer rows.Close()

	var movies []*storage.ActiveMovie
	for rows.Next() {
		var movie storage.ActiveMovie
		var images sql.NullString
		var downloadID sql.NullString
		var dcID sql.NullInt32
		var dcHost, dcPort sql.NullString

		if err := rows.Scan(
			&movie.ID,
			&movie.TMDBID,
			&movie.Title,
			&movie.Year,
			&images,
			&movie.State,
			&movie.StateSince,
			&downloadID,
			&dcID,
			&dcHost,
			&dcPort,
		); err != nil {
			return nil, err
		}

		if images.Valid {
			movie.PosterPath = images.String
		}
		if downloadID.Valid {
			movie.DownloadID = downloadID.String
		}
		if dcID.Valid && dcHost.Valid && dcPort.Valid {
			var port int
			_, err := fmt.Sscanf(dcPort.String, "%d", &port)
			if err == nil {
				movie.DownloadClient = &storage.DownloadClientInfo{
					ID:   int(dcID.Int32),
					Host: dcHost.String,
					Port: port,
				}
			}
		}

		movies = append(movies, &movie)
	}

	return movies, nil
}

func (s *SQLite) ListDownloadingSeries(ctx context.Context) ([]*storage.ActiveSeries, error) {
	query := `
		SELECT
			se.id,
			sm.tmdb_id,
			sm.title,
			sm.poster_path,
			st.to_state,
			st.sort_key,
			st.download_id,
			se.season_number,
			dc.id,
			dc.host,
			dc.port
		FROM season se
		INNER JOIN season_transition st ON se.id = st.season_id AND st.most_recent = 1
		INNER JOIN series ser ON se.series_id = ser.id
		LEFT JOIN series_metadata sm ON ser.series_metadata_id = sm.id
		LEFT JOIN download_client dc ON st.download_client_id = dc.id
		WHERE st.to_state = 'downloading'
		ORDER BY st.sort_key DESC
	`

	s.mu.Lock()
	rows, err := s.db.QueryContext(ctx, query)
	s.mu.Unlock()

	if err != nil {
		return nil, fmt.Errorf("failed to query downloading series: %w", err)
	}
	defer rows.Close()

	var series []*storage.ActiveSeries
	for rows.Next() {
		var s storage.ActiveSeries
		var posterPath sql.NullString
		var downloadID sql.NullString
		var dcID sql.NullInt32
		var dcHost, dcPort sql.NullString
		var seasonNum sql.NullInt32

		if err := rows.Scan(
			&s.ID,
			&s.TMDBID,
			&s.Title,
			&posterPath,
			&s.State,
			&s.StateSince,
			&downloadID,
			&seasonNum,
			&dcID,
			&dcHost,
			&dcPort,
		); err != nil {
			return nil, err
		}

		if posterPath.Valid {
			s.PosterPath = posterPath.String
		}
		if downloadID.Valid {
			s.DownloadID = downloadID.String
		}
		if seasonNum.Valid {
			sn := int(seasonNum.Int32)
			s.SeasonNumber = &sn
		}
		if dcID.Valid && dcHost.Valid && dcPort.Valid {
			var port int
			_, err := fmt.Sscanf(dcPort.String, "%d", &port)
			if err == nil {
				s.DownloadClient = &storage.DownloadClientInfo{
					ID:   int(dcID.Int32),
					Host: dcHost.String,
					Port: port,
				}
			}
		}

		series = append(series, &s)
	}

	return series, nil
}

func (s *SQLite) ListRunningJobs(ctx context.Context) ([]*storage.ActiveJob, error) {
	query := `
		SELECT
			j.id,
			j.type,
			jt.to_state,
			j.created_at,
			jt.updated_at
		FROM job j
		INNER JOIN job_transition jt ON j.id = jt.job_id AND jt.most_recent = 1
		WHERE jt.to_state = 'running'
		ORDER BY j.created_at DESC
	`

	s.mu.Lock()
	rows, err := s.db.QueryContext(ctx, query)
	s.mu.Unlock()

	if err != nil {
		return nil, fmt.Errorf("failed to query running jobs: %w", err)
	}
	defer rows.Close()

	var jobs []*storage.ActiveJob
	for rows.Next() {
		var job storage.ActiveJob
		if err := rows.Scan(&job.ID, &job.Type, &job.State, &job.CreatedAt, &job.UpdatedAt); err != nil {
			return nil, err
		}
		jobs = append(jobs, &job)
	}

	return jobs, nil
}

func (s *SQLite) ListErrorJobs(ctx context.Context, hours int) ([]*storage.ActiveJob, error) {
	query := `
		SELECT
			j.id,
			j.type,
			jt.to_state,
			j.created_at,
			jt.updated_at
		FROM job j
		INNER JOIN job_transition jt ON j.id = jt.job_id AND jt.most_recent = 1
		WHERE jt.to_state = 'error'
		  AND jt.updated_at >= datetime('now', '-' || ? || ' hours')
		ORDER BY jt.updated_at DESC
	`

	s.mu.Lock()
	rows, err := s.db.QueryContext(ctx, query, hours)
	s.mu.Unlock()

	if err != nil {
		return nil, fmt.Errorf("failed to query error jobs: %w", err)
	}
	defer rows.Close()

	var jobs []*storage.ActiveJob
	for rows.Next() {
		var job storage.ActiveJob
		if err := rows.Scan(&job.ID, &job.Type, &job.State, &job.CreatedAt, &job.UpdatedAt); err != nil {
			return nil, err
		}
		jobs = append(jobs, &job)
	}

	return jobs, nil
}

func (s *SQLite) CountTransitionsByDate(ctx context.Context, startDate, endDate time.Time) (int, error) {
	startDateStr := startDate.Format("2006-01-02 15:04:05")
	endDateStr := endDate.Format("2006-01-02 15:04:05")

	s.mu.Lock()
	defer s.mu.Unlock()

	var totalCount int64
	err := s.db.QueryRowContext(ctx, `
		SELECT (
			SELECT COUNT(*) FROM movie_transition mt
			INNER JOIN movie m ON m.id = mt.movie_id
			WHERE mt.created_at >= datetime(?) AND mt.created_at <= datetime(?)
			  AND mt.most_recent = 1
		) + (
			SELECT COUNT(*) FROM season_transition st
			INNER JOIN season s ON s.id = st.season_id
			WHERE st.created_at >= datetime(?) AND st.created_at <= datetime(?)
			  AND st.most_recent = 1
		) + (
			SELECT COUNT(*) FROM episode_transition et
			INNER JOIN episode e ON e.id = et.episode_id
			WHERE et.created_at >= datetime(?) AND et.created_at <= datetime(?)
			  AND et.most_recent = 1
		) + (
			SELECT COUNT(*) FROM job_transition jt
			INNER JOIN job j ON j.id = jt.job_id
			WHERE jt.created_at >= datetime(?) AND jt.created_at <= datetime(?)
			  AND jt.most_recent = 1
		) AS total
	`, startDateStr, endDateStr, startDateStr, endDateStr, startDateStr, endDateStr, startDateStr, endDateStr).Scan(&totalCount)

	return int(totalCount), err
}

func (s *SQLite) GetTransitionsByDate(ctx context.Context, startDate, endDate time.Time, offset, limit int) (*storage.TimelineResponse, error) {
	startDateStr := startDate.Format("2006-01-02 15:04:05")
	endDateStr := endDate.Format("2006-01-02 15:04:05")

	s.mu.Lock()
	moviesRows, err := s.db.QueryContext(ctx, `
		SELECT
			STRFTIME('%Y-%m-%d', mt.created_at) AS date,
			COUNT(DISTINCT CASE WHEN mt.to_state = 'downloaded' THEN m.id END) AS downloaded,
			COUNT(DISTINCT CASE WHEN mt.to_state = 'downloading' THEN m.id END) AS downloading
		FROM movie m
		INNER JOIN movie_transition mt ON m.id = mt.movie_id
		WHERE mt.created_at >= datetime(?)
		  AND mt.created_at <= datetime(?)
		  AND mt.most_recent = 1
		GROUP BY STRFTIME('%Y-%m-%d', mt.created_at)
		ORDER BY date
	`, startDateStr, endDateStr)
	s.mu.Unlock()

	if err != nil {
		return nil, fmt.Errorf("failed to get movie transitions: %w", err)
	}
	defer moviesRows.Close()

	var movieEntries []*movieTimelineEntry
	for moviesRows.Next() {
		var entry movieTimelineEntry
		if err := moviesRows.Scan(&entry.Date, &entry.Downloaded, &entry.Downloading); err != nil {
			return nil, err
		}
		movieEntries = append(movieEntries, &entry)
	}

	s.mu.Lock()
	seriesRows, err := s.db.QueryContext(ctx, `
		SELECT
			STRFTIME('%Y-%m-%d', st.created_at) AS date,
			COUNT(DISTINCT CASE WHEN st.to_state = 'completed' THEN ser.id END) AS completed,
			COUNT(DISTINCT CASE WHEN st.to_state = 'downloading' THEN ser.id END) AS downloading
		FROM season s
		INNER JOIN season_transition st ON s.id = st.season_id
		INNER JOIN series ser ON s.series_id = ser.id
		WHERE st.created_at >= datetime(?)
		  AND st.created_at <= datetime(?)
		  AND st.most_recent = 1
		GROUP BY STRFTIME('%Y-%m-%d', st.created_at)
		ORDER BY date
	`, startDateStr, endDateStr)
	s.mu.Unlock()

	if err != nil {
		return nil, fmt.Errorf("failed to get series transitions: %w", err)
	}
	defer seriesRows.Close()

	var seriesEntries []*seriesTimelineEntry
	for seriesRows.Next() {
		var entry seriesTimelineEntry
		if err := seriesRows.Scan(&entry.Date, &entry.Completed, &entry.Downloading); err != nil {
			return nil, err
		}
		seriesEntries = append(seriesEntries, &entry)
	}

	s.mu.Lock()
	jobsRows, err := s.db.QueryContext(ctx, `
		SELECT
			STRFTIME('%Y-%m-%d', jt.created_at) AS date,
			COUNT(DISTINCT CASE WHEN jt.to_state = 'done' THEN j.id END) AS done,
			COUNT(DISTINCT CASE WHEN jt.to_state = 'error' THEN j.id END) AS error
		FROM job j
		INNER JOIN job_transition jt ON j.id = jt.job_id
		WHERE jt.created_at >= datetime(?)
		  AND jt.created_at <= datetime(?)
		  AND jt.most_recent = 1
		GROUP BY STRFTIME('%Y-%m-%d', jt.created_at)
		ORDER BY date
	`, startDateStr, endDateStr)
	s.mu.Unlock()

	if err != nil {
		return nil, fmt.Errorf("failed to get job transitions: %w", err)
	}
	defer jobsRows.Close()

	var jobEntries []*jobTimelineEntry
	for jobsRows.Next() {
		var entry jobTimelineEntry
		if err := jobsRows.Scan(&entry.Date, &entry.Done, &entry.Error); err != nil {
			return nil, err
		}
		jobEntries = append(jobEntries, &entry)
	}

	timelineMap := make(map[string]*storage.TimelineEntry)

	for _, entry := range movieEntries {
		if _, exists := timelineMap[entry.Date]; !exists {
			timelineMap[entry.Date] = &storage.TimelineEntry{
				Date:   entry.Date,
				Movies: &storage.MovieCounts{},
				Series: &storage.SeriesCounts{},
				Jobs:   &storage.JobCounts{},
			}
		}
		timelineMap[entry.Date].Movies.Downloaded += entry.Downloaded
		timelineMap[entry.Date].Movies.Downloading += entry.Downloading
	}

	for _, entry := range seriesEntries {
		if _, exists := timelineMap[entry.Date]; !exists {
			timelineMap[entry.Date] = &storage.TimelineEntry{
				Date:   entry.Date,
				Movies: &storage.MovieCounts{},
				Series: &storage.SeriesCounts{},
				Jobs:   &storage.JobCounts{},
			}
		}
		timelineMap[entry.Date].Series.Completed += entry.Completed
		timelineMap[entry.Date].Series.Downloading += entry.Downloading
	}

	for _, entry := range jobEntries {
		if _, exists := timelineMap[entry.Date]; !exists {
			timelineMap[entry.Date] = &storage.TimelineEntry{
				Date:   entry.Date,
				Movies: &storage.MovieCounts{},
				Series: &storage.SeriesCounts{},
				Jobs:   &storage.JobCounts{},
			}
		}
		timelineMap[entry.Date].Jobs.Done += entry.Done
		timelineMap[entry.Date].Jobs.Error += entry.Error
	}

	var timeline []*storage.TimelineEntry
	for _, entry := range timelineMap {
		timeline = append(timeline, entry)
	}

	s.mu.Lock()
	var movieTransitionRows *sql.Rows
	if limit > 0 {
		movieTransitionQuery := `
			SELECT
				mt.id,
				'movie' AS entity_type,
				m.id AS entity_id,
				COALESCE(mm.title, '') AS entity_title,
				mt.to_state,
				mt.from_state,
				mt.created_at
			FROM movie m
			INNER JOIN movie_transition mt ON m.id = mt.movie_id
			LEFT JOIN movie_metadata mm ON m.movie_metadata_id = mm.id
			WHERE mt.created_at >= datetime(?)
			  AND mt.created_at <= datetime(?)
			  AND mt.most_recent = 1
			ORDER BY mt.created_at DESC
			LIMIT ? OFFSET ?
		`
		movieTransitionRows, err = s.db.QueryContext(ctx, movieTransitionQuery, startDateStr, endDateStr, limit, offset)
	} else {
		movieTransitionQuery := `
			SELECT
				mt.id,
				'movie' AS entity_type,
				m.id AS entity_id,
				COALESCE(mm.title, '') AS entity_title,
				mt.to_state,
				mt.from_state,
				mt.created_at
			FROM movie m
			INNER JOIN movie_transition mt ON m.id = mt.movie_id
			LEFT JOIN movie_metadata mm ON m.movie_metadata_id = mm.id
			WHERE mt.created_at >= datetime(?)
			  AND mt.created_at <= datetime(?)
			  AND mt.most_recent = 1
			ORDER BY mt.created_at DESC
		`
		movieTransitionRows, err = s.db.QueryContext(ctx, movieTransitionQuery, startDateStr, endDateStr)
	}
	s.mu.Unlock()

	if err != nil {
		return nil, fmt.Errorf("failed to get movie transition items: %w", err)
	}
	defer movieTransitionRows.Close()

	var transitions []*storage.TransitionItem

	for movieTransitionRows.Next() {
		var fromState sql.NullString
		var item storage.TransitionItem
		if err := movieTransitionRows.Scan(&item.ID, &item.EntityType, &item.EntityID, &item.EntityTitle, &item.ToState, &fromState, &item.CreatedAt); err != nil {
			return nil, err
		}
		if fromState.Valid {
			item.FromState = &fromState.String
		}
		transitions = append(transitions, &item)
	}

	s.mu.Lock()
	var seasonTransitionRows *sql.Rows
	if limit > 0 {
		seasonTransitionQuery := `
			SELECT
				st.id,
				'season' AS entity_type,
				s.id AS entity_id,
				COALESCE(sm.title, '') AS entity_title,
				st.to_state,
				st.from_state,
				st.created_at
			FROM season s
			INNER JOIN season_transition st ON s.id = st.season_id
			INNER JOIN series ser ON s.series_id = ser.id
			LEFT JOIN series_metadata sm ON ser.series_metadata_id = sm.id
			WHERE st.created_at >= datetime(?)
			  AND st.created_at <= datetime(?)
			  AND st.most_recent = 1
			ORDER BY st.created_at DESC
			LIMIT ? OFFSET ?
		`
		seasonTransitionRows, err = s.db.QueryContext(ctx, seasonTransitionQuery, startDateStr, endDateStr, limit, offset)
	} else {
		seasonTransitionQuery := `
			SELECT
				st.id,
				'season' AS entity_type,
				s.id AS entity_id,
				COALESCE(sm.title, '') AS entity_title,
				st.to_state,
				st.from_state,
				st.created_at
			FROM season s
			INNER JOIN season_transition st ON s.id = st.season_id
			INNER JOIN series ser ON s.series_id = ser.id
			LEFT JOIN series_metadata sm ON ser.series_metadata_id = sm.id
			WHERE st.created_at >= datetime(?)
			  AND st.created_at <= datetime(?)
			  AND st.most_recent = 1
			ORDER BY st.created_at DESC
		`
		seasonTransitionRows, err = s.db.QueryContext(ctx, seasonTransitionQuery, startDateStr, endDateStr)
	}
	s.mu.Unlock()

	if err != nil {
		return nil, fmt.Errorf("failed to get season transition items: %w", err)
	}
	defer seasonTransitionRows.Close()

	for seasonTransitionRows.Next() {
		var fromState sql.NullString
		var item storage.TransitionItem
		if err := seasonTransitionRows.Scan(&item.ID, &item.EntityType, &item.EntityID, &item.EntityTitle, &item.ToState, &fromState, &item.CreatedAt); err != nil {
			return nil, err
		}
		if fromState.Valid {
			item.FromState = &fromState.String
		}
		transitions = append(transitions, &item)
	}

	s.mu.Lock()
	var episodeTransitionRows *sql.Rows
	if limit > 0 {
		episodeTransitionQuery := `
			SELECT
				et.id,
				'episode' AS entity_type,
				e.id AS entity_id,
				COALESCE(sm.title, '') AS entity_title,
				et.to_state,
				et.from_state,
				et.created_at
			FROM episode e
			INNER JOIN episode_transition et ON e.id = et.episode_id
			INNER JOIN season s ON e.season_id = s.id
			INNER JOIN series ser ON s.series_id = ser.id
			LEFT JOIN series_metadata sm ON ser.series_metadata_id = sm.id
			WHERE et.created_at >= datetime(?)
			  AND et.created_at <= datetime(?)
			  AND et.most_recent = 1
			ORDER BY et.created_at DESC
			LIMIT ? OFFSET ?
		`
		episodeTransitionRows, err = s.db.QueryContext(ctx, episodeTransitionQuery, startDateStr, endDateStr, limit, offset)
	} else {
		episodeTransitionQuery := `
			SELECT
				et.id,
				'episode' AS entity_type,
				e.id AS entity_id,
				COALESCE(sm.title, '') AS entity_title,
				et.to_state,
				et.from_state,
				et.created_at
			FROM episode e
			INNER JOIN episode_transition et ON e.id = et.episode_id
			INNER JOIN season s ON e.season_id = s.id
			INNER JOIN series ser ON s.series_id = ser.id
			LEFT JOIN series_metadata sm ON ser.series_metadata_id = sm.id
			WHERE et.created_at >= datetime(?)
			  AND et.created_at <= datetime(?)
			  AND et.most_recent = 1
			ORDER BY et.created_at DESC
		`
		episodeTransitionRows, err = s.db.QueryContext(ctx, episodeTransitionQuery, startDateStr, endDateStr)
	}
	s.mu.Unlock()

	if err != nil {
		return nil, fmt.Errorf("failed to get episode transition items: %w", err)
	}
	defer episodeTransitionRows.Close()

	for episodeTransitionRows.Next() {
		var fromState sql.NullString
		var item storage.TransitionItem
		if err := episodeTransitionRows.Scan(&item.ID, &item.EntityType, &item.EntityID, &item.EntityTitle, &item.ToState, &fromState, &item.CreatedAt); err != nil {
			return nil, err
		}
		if fromState.Valid {
			item.FromState = &fromState.String
		}
		transitions = append(transitions, &item)
	}

	s.mu.Lock()
	var jobTransitionRows *sql.Rows
	if limit > 0 {
		jobTransitionQuery := `
			SELECT
				jt.id,
				'job' AS entity_type,
				j.id AS entity_id,
				j.type AS entity_title,
				jt.to_state,
				jt.from_state,
				jt.created_at
			FROM job j
			INNER JOIN job_transition jt ON j.id = jt.job_id
			WHERE jt.created_at >= datetime(?)
			  AND jt.created_at <= datetime(?)
			  AND jt.most_recent = 1
			ORDER BY jt.created_at DESC
			LIMIT ? OFFSET ?
		`
		jobTransitionRows, err = s.db.QueryContext(ctx, jobTransitionQuery, startDateStr, endDateStr, limit, offset)
	} else {
		jobTransitionQuery := `
			SELECT
				jt.id,
				'job' AS entity_type,
				j.id AS entity_id,
				j.type AS entity_title,
				jt.to_state,
				jt.from_state,
				jt.created_at
			FROM job j
			INNER JOIN job_transition jt ON j.id = jt.job_id
			WHERE jt.created_at >= datetime(?)
			  AND jt.created_at <= datetime(?)
			  AND jt.most_recent = 1
			ORDER BY jt.created_at DESC
		`
		jobTransitionRows, err = s.db.QueryContext(ctx, jobTransitionQuery, startDateStr, endDateStr)
	}
	s.mu.Unlock()

	if err != nil {
		return nil, fmt.Errorf("failed to get job transition items: %w", err)
	}
	defer jobTransitionRows.Close()

	for jobTransitionRows.Next() {
		var fromState sql.NullString
		var item storage.TransitionItem
		if err := jobTransitionRows.Scan(&item.ID, &item.EntityType, &item.EntityID, &item.EntityTitle, &item.ToState, &fromState, &item.CreatedAt); err != nil {
			return nil, err
		}
		if fromState.Valid {
			item.FromState = &fromState.String
		}
		transitions = append(transitions, &item)
	}

	totalCount, countErr := s.CountTransitionsByDate(ctx, startDate, endDate)
	if countErr != nil {
		return nil, countErr
	}

	return &storage.TimelineResponse{
		Timeline:    timeline,
		Transitions: transitions,
		Count:       totalCount,
	}, nil
}

func (s *SQLite) GetEntityTransitions(ctx context.Context, entityType string, entityID int64) (*storage.HistoryResponse, error) {
	var rows *sql.Rows
	var err error
	var history []*storage.HistoryEntry
	var entity *storage.EntityInfo

	switch entityType {
	case "movie":
		s.mu.Lock()
		rows, err = s.db.QueryContext(ctx, `
			SELECT
				mm.title AS entity_title,
				mm.images AS poster_path,
				mt.to_state,
				mt.from_state,
				mt.created_at,
				mt.sort_key,
				json_object(
					'download_client', json_object('id', dc.id, 'host', dc.host, 'port', dc.port),
					'download_id', mt.download_id
				) AS metadata
			FROM movie m
			INNER JOIN movie_transition mt ON m.id = mt.movie_id
			LEFT JOIN movie_metadata mm ON m.movie_metadata_id = mm.id
			LEFT JOIN download_client dc ON mt.download_client_id = dc.id
			WHERE m.id = ?
			ORDER BY mt.sort_key ASC
		`, entityID)
		s.mu.Unlock()
		if err != nil {
			return nil, fmt.Errorf("failed to get movie transitions: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var metadataJSON sql.NullString
			var item storage.HistoryEntry
			var posterPath sql.NullString
			var entityTitle sql.NullString
			if err := rows.Scan(&entityTitle, &posterPath, &item.ToState, &item.FromState, &item.CreatedAt, &item.SortKey, &metadataJSON); err != nil {
				return nil, err
			}
			if entity == nil && posterPath.Valid && entityTitle.Valid {
				entity = &storage.EntityInfo{
					Type:       "movie",
					ID:         entityID,
					Title:      entityTitle.String,
					PosterPath: posterPath.String,
				}
			}
			if metadataJSON.Valid && metadataJSON.String != "" {
				item.Metadata = &storage.TransitionMetadata{}
				if err := json.Unmarshal([]byte(metadataJSON.String), item.Metadata); err != nil {
					return nil, err
				}
			}
			history = append(history, &item)
		}

	case "series":
		s.mu.Lock()
		rows, err = s.db.QueryContext(ctx, `
			SELECT
				sm.title AS entity_title,
				sm.poster_path AS poster_path,
				st.to_state,
				st.from_state,
				st.created_at,
				st.sort_key,
				json_object(
					'download_client', json_object('id', dc.id, 'host', dc.host, 'port', dc.port),
					'download_id', st.download_id
				) AS metadata
			FROM series s
			INNER JOIN series_transition st ON s.id = st.series_id
			LEFT JOIN series_metadata sm ON s.series_metadata_id = sm.id
			LEFT JOIN download_client dc ON st.download_client_id = dc.id
			WHERE s.id = ?
			ORDER BY st.sort_key ASC
		`, entityID)
		s.mu.Unlock()
		if err != nil {
			return nil, fmt.Errorf("failed to get series transitions: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var metadataJSON sql.NullString
			var item storage.HistoryEntry
			var posterPath sql.NullString
			var entityTitle sql.NullString
			if err := rows.Scan(&entityTitle, &posterPath, &item.ToState, &item.FromState, &item.CreatedAt, &item.SortKey, &metadataJSON); err != nil {
				return nil, err
			}
			if entity == nil && posterPath.Valid && entityTitle.Valid {
				entity = &storage.EntityInfo{
					Type:       "series",
					ID:         entityID,
					Title:      entityTitle.String,
					PosterPath: posterPath.String,
				}
			}
			if metadataJSON.Valid && metadataJSON.String != "" {
				item.Metadata = &storage.TransitionMetadata{}
				if err := json.Unmarshal([]byte(metadataJSON.String), item.Metadata); err != nil {
					return nil, err
				}
			}
			history = append(history, &item)
		}

	case "season":
		s.mu.Lock()
		rows, err = s.db.QueryContext(ctx, `
			SELECT
				sm.title AS entity_title,
				sm.poster_path AS poster_path,
				st.to_state,
				st.from_state,
				st.created_at,
				st.sort_key,
				json_object(
					'download_client', json_object('id', dc.id, 'host', dc.host, 'port', dc.port),
					'download_id', st.download_id
				) AS metadata
			FROM season s
			INNER JOIN season_transition st ON s.id = st.season_id
			INNER JOIN series ser ON s.series_id = ser.id
			LEFT JOIN series_metadata sm ON ser.series_metadata_id = sm.id
			LEFT JOIN download_client dc ON st.download_client_id = dc.id
			WHERE s.id = ?
			ORDER BY st.sort_key ASC
		`, entityID)
		s.mu.Unlock()
		if err != nil {
			return nil, fmt.Errorf("failed to get season transitions: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var metadataJSON sql.NullString
			var item storage.HistoryEntry
			var posterPath sql.NullString
			var entityTitle sql.NullString
			if err := rows.Scan(&entityTitle, &posterPath, &item.ToState, &item.FromState, &item.CreatedAt, &item.SortKey, &metadataJSON); err != nil {
				return nil, err
			}
			if entity == nil && posterPath.Valid && entityTitle.Valid {
				entity = &storage.EntityInfo{
					Type:       "season",
					ID:         entityID,
					Title:      entityTitle.String,
					PosterPath: posterPath.String,
				}
			}
			if metadataJSON.Valid && metadataJSON.String != "" {
				item.Metadata = &storage.TransitionMetadata{}
				if err := json.Unmarshal([]byte(metadataJSON.String), item.Metadata); err != nil {
					return nil, err
				}
			}
			history = append(history, &item)
		}

	case "episode":
		s.mu.Lock()
		rows, err = s.db.QueryContext(ctx, `
			SELECT
				sm.title AS entity_title,
				sm.poster_path AS poster_path,
				et.to_state,
				et.from_state,
				et.created_at,
				et.sort_key
			FROM episode e
			INNER JOIN episode_transition et ON e.id = et.episode_id
			INNER JOIN season s ON e.season_id = s.id
			INNER JOIN series ser ON s.series_id = ser.id
			LEFT JOIN series_metadata sm ON ser.series_metadata_id = sm.id
			WHERE e.id = ?
			ORDER BY et.sort_key ASC
		`, entityID)
		s.mu.Unlock()
		if err != nil {
			return nil, fmt.Errorf("failed to get episode transitions: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var item storage.HistoryEntry
			var posterPath sql.NullString
			var entityTitle sql.NullString
			if err := rows.Scan(&entityTitle, &posterPath, &item.ToState, &item.FromState, &item.CreatedAt, &item.SortKey); err != nil {
				return nil, err
			}
			if entity == nil && posterPath.Valid && entityTitle.Valid {
				entity = &storage.EntityInfo{
					Type:       "episode",
					ID:         entityID,
					Title:      entityTitle.String,
					PosterPath: posterPath.String,
				}
			}
			history = append(history, &item)
		}

	case "job":
		s.mu.Lock()
		rows, err = s.db.QueryContext(ctx, `
			SELECT
				j.type AS entity_title,
				'' AS poster_path,
				jt.to_state,
				jt.from_state,
				jt.created_at,
				jt.sort_key
		FROM job j
		INNER JOIN job_transition jt ON j.id = jt.job_id
		WHERE j.id = ?
		ORDER BY jt.sort_key ASC
	`, entityID)
		s.mu.Unlock()
		if err != nil {
			return nil, fmt.Errorf("failed to get job transitions: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var item storage.HistoryEntry
			var posterPath sql.NullString
			var entityTitle sql.NullString
			if err := rows.Scan(&entityTitle, &posterPath, &item.ToState, &item.FromState, &item.CreatedAt, &item.SortKey); err != nil {
				return nil, err
			}
			if entity == nil && entityTitle.Valid {
				entity = &storage.EntityInfo{
					Type:  "job",
					ID:    entityID,
					Title: entityTitle.String,
				}
			}
			history = append(history, &item)
		}

	default:
		return nil, fmt.Errorf("unsupported entity type: %s", entityType)
	}

	return &storage.HistoryResponse{
		Entity:  entity,
		History: history,
	}, nil
}
