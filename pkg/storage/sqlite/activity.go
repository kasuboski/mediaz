package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/kasuboski/mediaz/pkg/storage"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
)

func (s *SQLite) ListDownloadingMovies(ctx context.Context) ([]*storage.ActiveMovie, error) {
	rows, err := s.Queries.ListDownloadingMovies(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query downloading movies: %w", err)
	}

	movies := make([]*storage.ActiveMovie, 0, len(rows))
	for _, r := range rows {
		movie := &storage.ActiveMovie{
			ID:         r.ID,
			TMDBID:     r.TmdbID,
			Title:      r.Title,
			State:      r.ToState,
			StateSince: time.Unix(r.SortKey, 0),
			PosterPath: r.Images,
		}
		if r.Year.Valid {
			movie.Year = int(r.Year.Int64)
		}
		if r.DownloadID.Valid {
			movie.DownloadID = r.DownloadID.String
		}
		if r.DcID.Valid && r.DcHost.Valid && r.DcPort.Valid {
			movie.DownloadClient = &storage.DownloadClientInfo{
				ID:   int(r.DcID.Int64),
				Host: r.DcHost.String,
				Port: int(r.DcPort.Int64),
			}
		}
		movies = append(movies, movie)
	}

	return movies, nil
}

func (s *SQLite) ListDownloadingSeries(ctx context.Context) ([]*storage.ActiveSeries, error) {
	rows, err := s.Queries.ListDownloadingSeries(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query downloading series: %w", err)
	}

	series := make([]*storage.ActiveSeries, 0, len(rows))
	for _, r := range rows {
		s := &storage.ActiveSeries{
			ID:         r.ID,
			State:      r.ToState,
			StateSince: time.Unix(r.SortKey, 0),
		}
		if r.TmdbID.Valid {
			s.TMDBID = r.TmdbID.Int64
		}
		if r.Title.Valid {
			s.Title = r.Title.String
		}
		if r.PosterPath.Valid {
			s.PosterPath = r.PosterPath.String
		}
		if r.DownloadID.Valid {
			s.DownloadID = r.DownloadID.String
		}
		sn := int(r.SeasonNumber)
		s.SeasonNumber = &sn
		if r.DcID.Valid && r.DcHost.Valid && r.DcPort.Valid {
			s.DownloadClient = &storage.DownloadClientInfo{
				ID:   int(r.DcID.Int64),
				Host: r.DcHost.String,
				Port: int(r.DcPort.Int64),
			}
		}
		series = append(series, s)
	}

	return series, nil
}

func (s *SQLite) ListRunningJobs(ctx context.Context) ([]*storage.ActiveJob, error) {
	rows, err := s.Queries.ListRunningJobs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query running jobs: %w", err)
	}

	jobs := make([]*storage.ActiveJob, 0, len(rows))
	for _, r := range rows {
		j := &storage.ActiveJob{
			ID:    r.ID,
			Type:  r.Type,
			State: r.ToState,
		}
		if r.CreatedAt.Valid {
			j.CreatedAt = r.CreatedAt.Time
		}
		if r.UpdatedAt.Valid {
			j.UpdatedAt = r.UpdatedAt.Time
		}
		jobs = append(jobs, j)
	}

	return jobs, nil
}

func (s *SQLite) ListErrorJobs(ctx context.Context, hours int) ([]*storage.ActiveJob, error) {
	cutoff := time.Now().UTC().Add(-time.Duration(hours) * time.Hour)

	rows, err := s.Queries.ListErrorJobs(ctx, sql.NullTime{Time: cutoff, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("failed to query error jobs: %w", err)
	}

	jobs := make([]*storage.ActiveJob, 0, len(rows))
	for _, r := range rows {
		j := &storage.ActiveJob{
			ID:    r.ID,
			Type:  r.Type,
			State: r.ToState,
		}
		if r.CreatedAt.Valid {
			j.CreatedAt = r.CreatedAt.Time
		}
		if r.UpdatedAt.Valid {
			j.UpdatedAt = r.UpdatedAt.Time
		}
		jobs = append(jobs, j)
	}

	return jobs, nil
}

func (s *SQLite) CountTransitionsByDate(ctx context.Context, startDate, endDate time.Time) (int, error) {
	sd := startDate.Format(timestampFormat)
	ed := endDate.Format(timestampFormat)

	total, err := s.Queries.CountTransitionsByDate(ctx, sqlcdb.CountTransitionsByDateParams{
		Datetime:   sd,
		Datetime_2: ed,
		Datetime_3: sd,
		Datetime_4: ed,
		Datetime_5: sd,
		Datetime_6: ed,
		Datetime_7: sd,
		Datetime_8: ed,
	})
	if err != nil {
		return 0, err
	}

	return int(total), nil
}

func (s *SQLite) GetTransitionsByDate(ctx context.Context, startDate, endDate time.Time, offset, limit int) (*storage.TimelineResponse, error) {
	sd := startDate.Format(timestampFormat)
	ed := endDate.Format(timestampFormat)

	// --- Timeline aggregation ---

	movieRows, err := s.GetMovieTransitionsByDate(ctx, sqlcdb.GetMovieTransitionsByDateParams{
		Datetime:   sd,
		Datetime_2: ed,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get movie transitions: %w", err)
	}

	seriesRows, err := s.GetSeriesTransitionsByDate(ctx, sqlcdb.GetSeriesTransitionsByDateParams{
		Datetime:   sd,
		Datetime_2: ed,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get series transitions: %w", err)
	}

	jobRows, err := s.GetJobTransitionsByDate(ctx, sqlcdb.GetJobTransitionsByDateParams{
		Datetime:   sd,
		Datetime_2: ed,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get job transitions: %w", err)
	}

	timelineMap := make(map[string]*storage.TimelineEntry)

	for _, r := range movieRows {
		ensureTimelineEntry(timelineMap, r.Date)
		timelineMap[r.Date].Movies.Downloaded += int(r.Downloaded)
		timelineMap[r.Date].Movies.Downloading += int(r.Downloading)
	}

	for _, r := range seriesRows {
		ensureTimelineEntry(timelineMap, r.Date)
		timelineMap[r.Date].Series.Completed += int(r.Completed)
		timelineMap[r.Date].Series.Downloading += int(r.Downloading)
	}

	for _, r := range jobRows {
		ensureTimelineEntry(timelineMap, r.Date)
		timelineMap[r.Date].Jobs.Done += int(r.Done)
		timelineMap[r.Date].Jobs.Error += int(r.Error)
	}

	var timeline []*storage.TimelineEntry
	for _, entry := range timelineMap {
		timeline = append(timeline, entry)
	}
	sort.Slice(timeline, func(i, j int) bool {
		return timeline[i].Date < timeline[j].Date
	})

	// --- Transition items ---

	transitions, err := s.fetchAndSortTransitionItems(ctx, sd, ed)
	if err != nil {
		return nil, err
	}

	// Apply global pagination across all entity types.
	if limit > 0 {
		end := offset + limit
		if end > len(transitions) {
			end = len(transitions)
		}
		if offset > len(transitions) {
			offset = len(transitions)
		}
		transitions = transitions[offset:end]
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

// fetchAndSortTransitionItems retrieves transition items for all entity types
// and returns them sorted by CreatedAt descending.
func (s *SQLite) fetchAndSortTransitionItems(ctx context.Context, sd, ed string) ([]*storage.TransitionItem, error) {
	var transitions []*storage.TransitionItem

	movieItems, err := s.GetMovieTransitionItemsNoLimit(ctx, sqlcdb.GetMovieTransitionItemsNoLimitParams{
		Datetime: sd, Datetime_2: ed,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get movie transition items: %w", err)
	}
	for _, r := range movieItems {
		transitions = append(transitions, convertTransitionItemRow(r.ID, r.EntityType, r.EntityID, r.EntityTitle, r.ToState, r.FromState, r.CreatedAt))
	}

	seasonItems, err := s.GetSeasonTransitionItemsNoLimit(ctx, sqlcdb.GetSeasonTransitionItemsNoLimitParams{
		Datetime: sd, Datetime_2: ed,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get season transition items: %w", err)
	}
	for _, r := range seasonItems {
		transitions = append(transitions, convertTransitionItemRow(r.ID, r.EntityType, r.EntityID, r.EntityTitle, r.ToState, r.FromState, r.CreatedAt))
	}

	episodeItems, err := s.GetEpisodeTransitionItemsNoLimit(ctx, sqlcdb.GetEpisodeTransitionItemsNoLimitParams{
		Datetime: sd, Datetime_2: ed,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get episode transition items: %w", err)
	}
	for _, r := range episodeItems {
		transitions = append(transitions, convertTransitionItemRow(r.ID, r.EntityType, r.EntityID, r.EntityTitle, r.ToState, r.FromState, r.CreatedAt))
	}

	jobItems, err := s.GetJobTransitionItemsNoLimit(ctx, sqlcdb.GetJobTransitionItemsNoLimitParams{
		Datetime: sd, Datetime_2: ed,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get job transition items: %w", err)
	}
	for _, r := range jobItems {
		transitions = append(transitions, convertTransitionItemRow(r.ID, r.EntityType, r.EntityID, r.EntityTitle, r.ToState, r.FromState, r.CreatedAt))
	}

	sort.Slice(transitions, func(i, j int) bool {
		return transitions[i].CreatedAt.After(transitions[j].CreatedAt)
	})

	return transitions, nil
}

func convertTransitionItemRow(id int64, entityType string, entityID int64, entityTitle string, toState string, fromState sql.NullString, createdAt sql.NullTime) *storage.TransitionItem {
	item := &storage.TransitionItem{
		ID:          id,
		EntityType:  entityType,
		EntityID:    entityID,
		EntityTitle: entityTitle,
		ToState:     toState,
	}
	if fromState.Valid {
		item.FromState = &fromState.String
	}
	if createdAt.Valid {
		item.CreatedAt = createdAt.Time
	}
	return item
}

func ensureTimelineEntry(m map[string]*storage.TimelineEntry, date string) {
	if _, exists := m[date]; !exists {
		m[date] = &storage.TimelineEntry{
			Date:   date,
			Movies: &storage.MovieCounts{},
			Series: &storage.SeriesCounts{},
			Jobs:   &storage.JobCounts{},
		}
	}
}

func (s *SQLite) GetEntityTransitions(ctx context.Context, entityType string, entityID int64) (*storage.HistoryResponse, error) {
	switch entityType {
	case "movie":
		return s.getEntityTransitionsMovie(ctx, entityID)
	case "series":
		return s.getEntityTransitionsSeries(ctx, entityID)
	case "season":
		return s.getEntityTransitionsSeason(ctx, entityID)
	case "episode":
		return s.getEntityTransitionsEpisode(ctx, entityID)
	case "job":
		return s.getEntityTransitionsJob(ctx, entityID)
	default:
		return nil, fmt.Errorf("unsupported entity type: %s", entityType)
	}
}

func (s *SQLite) getEntityTransitionsMovie(ctx context.Context, entityID int64) (*storage.HistoryResponse, error) {
	rows, err := s.GetEntityTransitionsMovie(ctx, entityID)
	if err != nil {
		return nil, fmt.Errorf("failed to get movie transitions: %w", err)
	}

	var entity *storage.EntityInfo
	history := make([]*storage.HistoryEntry, 0, len(rows))

	for _, r := range rows {
		if entity == nil && r.EntityTitle.Valid {
			entity = &storage.EntityInfo{
				Type:       "movie",
				ID:         entityID,
				Title:      r.EntityTitle.String,
				PosterPath: stringFromNullString(r.PosterPath),
			}
		}
		entry := &storage.HistoryEntry{
			SortKey: int(r.SortKey),
			ToState: r.ToState,
		}
		if r.FromState.Valid {
			entry.FromState = &r.FromState.String
		}
		if r.CreatedAt.Valid {
			entry.CreatedAt = r.CreatedAt.Time
		}
		if r.Metadata != "" {
			entry.Metadata = &storage.TransitionMetadata{}
			if err := json.Unmarshal([]byte(r.Metadata), entry.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal movie transition metadata: %w", err)
			}
		}
		history = append(history, entry)
	}

	return &storage.HistoryResponse{
		Entity:  entity,
		History: history,
	}, nil
}

func (s *SQLite) getEntityTransitionsSeries(ctx context.Context, entityID int64) (*storage.HistoryResponse, error) {
	rows, err := s.GetEntityTransitionsSeries(ctx, entityID)
	if err != nil {
		return nil, fmt.Errorf("failed to get series transitions: %w", err)
	}

	var entity *storage.EntityInfo
	history := make([]*storage.HistoryEntry, 0, len(rows))

	for _, r := range rows {
		if entity == nil && r.EntityTitle.Valid {
			entity = &storage.EntityInfo{
				Type:       "series",
				ID:         entityID,
				Title:      r.EntityTitle.String,
				PosterPath: stringFromNullString(r.PosterPath),
			}
		}
		entry := &storage.HistoryEntry{
			SortKey: int(r.SortKey),
			ToState: r.ToState,
		}
		if r.FromState.Valid {
			entry.FromState = &r.FromState.String
		}
		if r.CreatedAt.Valid {
			entry.CreatedAt = r.CreatedAt.Time
		}
		history = append(history, entry)
	}

	return &storage.HistoryResponse{
		Entity:  entity,
		History: history,
	}, nil
}

func (s *SQLite) getEntityTransitionsSeason(ctx context.Context, entityID int64) (*storage.HistoryResponse, error) {
	rows, err := s.GetEntityTransitionsSeason(ctx, entityID)
	if err != nil {
		return nil, fmt.Errorf("failed to get season transitions: %w", err)
	}

	var entity *storage.EntityInfo
	history := make([]*storage.HistoryEntry, 0, len(rows))

	for _, r := range rows {
		if entity == nil && r.EntityTitle.Valid {
			entity = &storage.EntityInfo{
				Type:       "season",
				ID:         entityID,
				Title:      r.EntityTitle.String,
				PosterPath: stringFromNullString(r.PosterPath),
			}
		}
		entry := &storage.HistoryEntry{
			SortKey: int(r.SortKey),
			ToState: r.ToState,
		}
		if r.FromState.Valid {
			entry.FromState = &r.FromState.String
		}
		if r.CreatedAt.Valid {
			entry.CreatedAt = r.CreatedAt.Time
		}
		if r.Metadata != "" {
			entry.Metadata = &storage.TransitionMetadata{}
			if err := json.Unmarshal([]byte(r.Metadata), entry.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal season transition metadata: %w", err)
			}
		}
		history = append(history, entry)
	}

	return &storage.HistoryResponse{
		Entity:  entity,
		History: history,
	}, nil
}

func (s *SQLite) getEntityTransitionsEpisode(ctx context.Context, entityID int64) (*storage.HistoryResponse, error) {
	rows, err := s.GetEntityTransitionsEpisode(ctx, entityID)
	if err != nil {
		return nil, fmt.Errorf("failed to get episode transitions: %w", err)
	}

	var entity *storage.EntityInfo
	history := make([]*storage.HistoryEntry, 0, len(rows))

	for _, r := range rows {
		if entity == nil && r.EntityTitle.Valid {
			entity = &storage.EntityInfo{
				Type:       "episode",
				ID:         entityID,
				Title:      r.EntityTitle.String,
				PosterPath: stringFromNullString(r.PosterPath),
			}
		}
		entry := &storage.HistoryEntry{
			SortKey: int(r.SortKey),
			ToState: r.ToState,
		}
		if r.FromState.Valid {
			entry.FromState = &r.FromState.String
		}
		if r.CreatedAt.Valid {
			entry.CreatedAt = r.CreatedAt.Time
		}
		history = append(history, entry)
	}

	return &storage.HistoryResponse{
		Entity:  entity,
		History: history,
	}, nil
}

func (s *SQLite) getEntityTransitionsJob(ctx context.Context, entityID int64) (*storage.HistoryResponse, error) {
	rows, err := s.GetEntityTransitionsJob(ctx, entityID)
	if err != nil {
		return nil, fmt.Errorf("failed to get job transitions: %w", err)
	}

	var entity *storage.EntityInfo
	history := make([]*storage.HistoryEntry, 0, len(rows))

	for _, r := range rows {
		if entity == nil {
			entity = &storage.EntityInfo{
				Type:  "job",
				ID:    entityID,
				Title: r.EntityTitle,
			}
		}
		entry := &storage.HistoryEntry{
			SortKey: int(r.SortKey),
			ToState: r.ToState,
		}
		if r.FromState.Valid {
			entry.FromState = &r.FromState.String
		}
		if r.CreatedAt.Valid {
			entry.CreatedAt = r.CreatedAt.Time
		}
		history = append(history, entry)
	}

	return &storage.HistoryResponse{
		Entity:  entity,
		History: history,
	}, nil
}

func stringFromNullString(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}
