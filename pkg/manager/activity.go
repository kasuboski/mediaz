package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"database/sql"
	"github.com/kasuboski/mediaz/pkg/logger"
	"github.com/kasuboski/mediaz/pkg/pagination"
	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
	"go.uber.org/zap"
	"sort"
)

type MovieMetadataImages struct {
	PosterPath string `json:"poster_path,omitempty"`
}

func (js *JobService) GetActiveActivity(ctx context.Context) (*ActiveActivityResponse, error) {
	log := logger.FromCtx(ctx)

	movies, err := js.activityStorage.ListDownloadingMovies(ctx)
	if err != nil {
		log.Error("failed to list downloading movies", zap.Error(err))
		return nil, err
	}

	series, err := js.activityStorage.ListDownloadingSeries(ctx)
	if err != nil {
		log.Error("failed to list downloading series", zap.Error(err))
		return nil, err
	}

	jobs, err := js.activityStorage.ListRunningJobs(ctx)
	if err != nil {
		log.Error("failed to list running jobs", zap.Error(err))
		return nil, err
	}

	response := &ActiveActivityResponse{
		Movies: transformActiveMovies(movies),
		Series: transformActiveSeries(series),
		Jobs:   transformActiveJobs(jobs),
	}

	return response, nil
}

func transformActiveMovies(rows []sqlcdb.ListDownloadingMoviesRow) []*ActiveMovie {
	result := make([]*ActiveMovie, 0, len(rows))
	for _, r := range rows {
		var stateSince time.Time
		if r.CreatedAt.Valid {
			stateSince = r.CreatedAt.Time
		}
		var images MovieMetadataImages
		_ = json.Unmarshal([]byte(r.Images), &images)
		m := &ActiveMovie{ID: r.ID, TMDBID: r.TmdbID, Title: r.Title, State: r.ToState, StateSince: stateSince, Duration: formatDuration(time.Since(stateSince)), PosterPath: images.PosterPath}
		if r.Year.Valid {
			m.Year = int(r.Year.Int64)
		}
		if r.DownloadID.Valid {
			m.DownloadID = r.DownloadID.String
		}
		m.DownloadClient = activityDownloadClient(r.DcID, r.DcHost, r.DcPort)
		result = append(result, m)
	}
	return result
}

func transformActiveSeries(rows []sqlcdb.ListDownloadingSeriesRow) []*ActiveSeries {
	result := make([]*ActiveSeries, 0, len(rows))
	for _, r := range rows {
		var stateSince time.Time
		if r.CreatedAt.Valid {
			stateSince = r.CreatedAt.Time
		}
		s := &ActiveSeries{ID: r.ID, State: r.ToState, StateSince: stateSince, Duration: formatDuration(time.Since(stateSince))}
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
		s.DownloadClient = activityDownloadClient(r.DcID, r.DcHost, r.DcPort)
		// The current query returns downloading seasons, not episodes.
		result = append(result, s)
	}
	return result
}

func transformActiveJobs(rows []sqlcdb.ListRunningJobsRow) []*ActiveJob {
	result := make([]*ActiveJob, 0, len(rows))
	for _, r := range rows {
		j := &ActiveJob{ID: r.ID, Type: r.Type, State: r.ToState}
		if r.CreatedAt.Valid {
			j.CreatedAt = r.CreatedAt.Time
		}
		if r.UpdatedAt.Valid {
			j.UpdatedAt = r.UpdatedAt.Time
		}
		j.Duration = formatDuration(time.Since(j.UpdatedAt))
		result = append(result, j)
	}
	return result
}

func activityDownloadClient(id sql.NullInt64, host sql.NullString, port sql.NullInt64) *DownloadClientInfo {
	if !id.Valid || !host.Valid || !port.Valid {
		return nil
	}
	return &DownloadClientInfo{ID: id.Int64, Host: host.String, Port: int(port.Int64)}
}

func (js *JobService) GetRecentFailures(ctx context.Context, hours int) (*FailuresResponse, error) {
	log := logger.FromCtx(ctx)

	jobs, err := js.activityStorage.ListErrorJobs(ctx, sql.NullTime{Time: time.Now().UTC().Add(-time.Duration(hours) * time.Hour), Valid: true})
	if err != nil {
		log.Error("failed to list error jobs", zap.Error(err))
		return nil, err
	}

	cutoff := time.Now().Add(-time.Duration(hours) * time.Hour)

	failures := make([]*FailureItem, 0, len(jobs))
	for _, job := range jobs {
		if job.UpdatedAt.Valid && job.UpdatedAt.Time.After(cutoff) {
			failures = append(failures, &FailureItem{
				Type:      "job",
				ID:        job.ID,
				Title:     job.Type,
				Subtitle:  "",
				State:     job.ToState,
				FailedAt:  job.UpdatedAt.Time,
				Error:     "",
				Retryable: true,
			})
		}
	}

	return &FailuresResponse{
		Failures: failures,
	}, nil
}

func (js *JobService) GetActivityTimeline(ctx context.Context, days int, params pagination.Params) (*TimelineResponse, error) {
	offset, limit := params.CalculateOffsetLimit()
	response, err := js.getTransitionsByDate(ctx, time.Now().AddDate(0, 0, -days), time.Now(), offset, limit)
	if err != nil {
		return nil, err
	}
	// Public activity responses historically serialize empty collections as [].
	if response.Timeline == nil {
		response.Timeline = []*TimelineEntry{}
	}
	if response.Transitions == nil {
		response.Transitions = []*TransitionItem{}
	}
	return response, nil
}

func (js *JobService) countTransitionsByDate(ctx context.Context, startDate, endDate time.Time) (int, error) {
	count, err := js.activityStorage.CountTransitionsByDate(ctx, sqlcdb.CountTransitionsByDateParams{Start: startDate.UTC().Format(activityTimestampFormat), End: endDate.UTC().Format(activityTimestampFormat)})
	return int(count), err
}

func (js *JobService) GetEntityTransitionHistory(ctx context.Context, entityType string, entityID int64) (*HistoryResponse, error) {
	response, err := js.getEntityTransitions(ctx, entityType, entityID)
	if err != nil {
		return nil, err
	}
	response.History = calculateDurations(response.History)
	return response, nil
}

func calculateDurations(entries []*HistoryEntry) []*HistoryEntry {
	for i, entry := range entries {
		if i < len(entries)-1 {
			entry.Duration = formatDuration(entries[i+1].CreatedAt.Sub(entry.CreatedAt))
		} else {
			entry.Duration = formatDuration(time.Since(entry.CreatedAt))
		}
	}
	return entries
}

const activityTimestampFormat = "2006-01-02 15:04:05"

func (js *JobService) getTransitionsByDate(ctx context.Context, startDate, endDate time.Time, offset, limit int) (*TimelineResponse, error) {
	if offset < 0 || limit < 0 {
		return nil, fmt.Errorf("offset and limit must be nonnegative")
	}
	sd := startDate.UTC().Format(activityTimestampFormat)
	ed := endDate.UTC().Format(activityTimestampFormat)

	// --- Timeline aggregation ---

	movieRows, err := js.activityStorage.GetMovieTransitionsByDate(ctx, sqlcdb.GetMovieTransitionsByDateParams{
		Start: sd,
		End:   ed,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get movie transitions: %w", err)
	}

	seriesRows, err := js.activityStorage.GetSeriesTransitionsByDate(ctx, sqlcdb.GetSeriesTransitionsByDateParams{
		Start: sd,
		End:   ed,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get series transitions: %w", err)
	}

	jobRows, err := js.activityStorage.GetJobTransitionsByDate(ctx, sqlcdb.GetJobTransitionsByDateParams{
		Start: sd,
		End:   ed,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get job transitions: %w", err)
	}

	timelineMap := make(map[string]*TimelineEntry)

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

	var timeline []*TimelineEntry
	for _, entry := range timelineMap {
		timeline = append(timeline, entry)
	}
	sort.Slice(timeline, func(i, j int) bool {
		return timeline[i].Date < timeline[j].Date
	})

	// --- Transition items ---

	transitions, err := js.fetchAndSortTransitionItems(ctx, sd, ed)
	if err != nil {
		return nil, err
	}

	// Apply global pagination across all entity types.
	if offset > len(transitions) {
		offset = len(transitions)
	}
	end := len(transitions)
	if limit > 0 && limit < end-offset {
		end = offset + limit
	}
	transitions = transitions[offset:end]

	totalCount, countErr := js.countTransitionsByDate(ctx, startDate, endDate)
	if countErr != nil {
		return nil, countErr
	}

	return &TimelineResponse{
		Timeline:    timeline,
		Transitions: transitions,
		Count:       totalCount,
	}, nil
}

// fetchAndSortTransitionItems retrieves transition items for all entity types
// (unlimited; the caller applies global pagination) and returns them sorted by
// CreatedAt descending.
func (js *JobService) fetchAndSortTransitionItems(ctx context.Context, sd, ed string) ([]*TransitionItem, error) {
	var transitions []*TransitionItem

	movieItems, err := js.activityStorage.GetMovieTransitionItems(ctx, sqlcdb.GetMovieTransitionItemsParams{
		Start: sd, End: ed, Offs: 0, Lim: 0,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get movie transition items: %w", err)
	}
	for _, r := range movieItems {
		transitions = append(transitions, convertTransitionItemRow(r.ID, r.EntityType, r.EntityID, r.EntityTitle, r.ToState, r.FromState, r.CreatedAt))
	}

	seasonItems, err := js.activityStorage.GetSeasonTransitionItems(ctx, sqlcdb.GetSeasonTransitionItemsParams{
		Start: sd, End: ed, Offs: 0, Lim: 0,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get season transition items: %w", err)
	}
	for _, r := range seasonItems {
		transitions = append(transitions, convertTransitionItemRow(r.ID, r.EntityType, r.EntityID, r.EntityTitle, r.ToState, r.FromState, r.CreatedAt))
	}

	episodeItems, err := js.activityStorage.GetEpisodeTransitionItems(ctx, sqlcdb.GetEpisodeTransitionItemsParams{
		Start: sd, End: ed, Offs: 0, Lim: 0,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get episode transition items: %w", err)
	}
	for _, r := range episodeItems {
		transitions = append(transitions, convertTransitionItemRow(r.ID, r.EntityType, r.EntityID, r.EntityTitle, r.ToState, r.FromState, r.CreatedAt))
	}

	jobItems, err := js.activityStorage.GetJobTransitionItems(ctx, sqlcdb.GetJobTransitionItemsParams{
		Start: sd, End: ed, Offs: 0, Lim: 0,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get job transition items: %w", err)
	}
	for _, r := range jobItems {
		transitions = append(transitions, convertTransitionItemRow(r.ID, r.EntityType, r.EntityID, r.EntityTitle, r.ToState, r.FromState, r.CreatedAt))
	}

	sort.Slice(transitions, func(i, j int) bool {
		if !transitions[i].CreatedAt.Equal(transitions[j].CreatedAt) {
			return transitions[i].CreatedAt.After(transitions[j].CreatedAt)
		}
		if transitions[i].EntityType != transitions[j].EntityType {
			return transitions[i].EntityType < transitions[j].EntityType
		}
		return transitions[i].ID > transitions[j].ID
	})

	return transitions, nil
}

func convertTransitionItemRow(id int64, entityType string, entityID int64, entityTitle string, toState string, fromState sql.NullString, createdAt sql.NullTime) *TransitionItem {
	item := &TransitionItem{
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

func ensureTimelineEntry(m map[string]*TimelineEntry, date string) {
	if _, exists := m[date]; !exists {
		m[date] = &TimelineEntry{
			Date:   date,
			Movies: &MovieCounts{},
			Series: &SeriesCounts{},
			Jobs:   &JobCounts{},
		}
	}
}

func (js *JobService) getEntityTransitions(ctx context.Context, entityType string, entityID int64) (*HistoryResponse, error) {
	switch entityType {
	case "movie":
		return js.getEntityTransitionsMovie(ctx, entityID)
	case "series":
		return js.getEntityTransitionsSeries(ctx, entityID)
	case "season":
		return js.getEntityTransitionsSeason(ctx, entityID)
	case "episode":
		return js.getEntityTransitionsEpisode(ctx, entityID)
	case "job":
		return js.getEntityTransitionsJob(ctx, entityID)
	default:
		return nil, fmt.Errorf("unsupported entity type: %s", entityType)
	}
}

func (js *JobService) getEntityTransitionsMovie(ctx context.Context, entityID int64) (*HistoryResponse, error) {
	rows, err := js.activityStorage.GetEntityTransitionsMovie(ctx, entityID)
	if err != nil {
		return nil, fmt.Errorf("failed to get movie transitions: %w", err)
	}

	var entity *EntityInfo
	history := make([]*HistoryEntry, 0, len(rows))

	for _, r := range rows {
		if entity == nil && r.EntityTitle.Valid {
			entity = &EntityInfo{
				Type:       "movie",
				ID:         entityID,
				Title:      r.EntityTitle.String,
				PosterPath: stringFromNullString(r.PosterPath),
			}
		}
		entry := &HistoryEntry{
			SortKey: r.SortKey,
			ToState: r.ToState,
		}
		if r.FromState.Valid {
			entry.FromState = &r.FromState.String
		}
		if r.CreatedAt.Valid {
			entry.CreatedAt = r.CreatedAt.Time
		}
		if r.Metadata != "" {
			entry.Metadata = &TransitionMetadata{}
			if err := json.Unmarshal([]byte(r.Metadata), entry.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal movie transition metadata: %w", err)
			}
		}
		history = append(history, entry)
	}

	return &HistoryResponse{
		Entity:  entity,
		History: history,
	}, nil
}

func (js *JobService) getEntityTransitionsSeries(ctx context.Context, entityID int64) (*HistoryResponse, error) {
	rows, err := js.activityStorage.GetEntityTransitionsSeries(ctx, entityID)
	if err != nil {
		return nil, fmt.Errorf("failed to get series transitions: %w", err)
	}

	var entity *EntityInfo
	history := make([]*HistoryEntry, 0, len(rows))

	for _, r := range rows {
		if entity == nil && r.EntityTitle.Valid {
			entity = &EntityInfo{
				Type:       "series",
				ID:         entityID,
				Title:      r.EntityTitle.String,
				PosterPath: stringFromNullString(r.PosterPath),
			}
		}
		entry := &HistoryEntry{
			SortKey: r.SortKey,
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

	return &HistoryResponse{
		Entity:  entity,
		History: history,
	}, nil
}

func (js *JobService) getEntityTransitionsSeason(ctx context.Context, entityID int64) (*HistoryResponse, error) {
	rows, err := js.activityStorage.GetEntityTransitionsSeason(ctx, entityID)
	if err != nil {
		return nil, fmt.Errorf("failed to get season transitions: %w", err)
	}

	var entity *EntityInfo
	history := make([]*HistoryEntry, 0, len(rows))

	for _, r := range rows {
		if entity == nil && r.EntityTitle.Valid {
			entity = &EntityInfo{
				Type:       "season",
				ID:         entityID,
				Title:      r.EntityTitle.String,
				PosterPath: stringFromNullString(r.PosterPath),
			}
		}
		entry := &HistoryEntry{
			SortKey: r.SortKey,
			ToState: r.ToState,
		}
		if r.FromState.Valid {
			entry.FromState = &r.FromState.String
		}
		if r.CreatedAt.Valid {
			entry.CreatedAt = r.CreatedAt.Time
		}
		if r.Metadata != "" {
			entry.Metadata = &TransitionMetadata{}
			if err := json.Unmarshal([]byte(r.Metadata), entry.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal season transition metadata: %w", err)
			}
		}
		history = append(history, entry)
	}

	return &HistoryResponse{
		Entity:  entity,
		History: history,
	}, nil
}

func (js *JobService) getEntityTransitionsEpisode(ctx context.Context, entityID int64) (*HistoryResponse, error) {
	rows, err := js.activityStorage.GetEntityTransitionsEpisode(ctx, entityID)
	if err != nil {
		return nil, fmt.Errorf("failed to get episode transitions: %w", err)
	}

	var entity *EntityInfo
	history := make([]*HistoryEntry, 0, len(rows))

	for _, r := range rows {
		if entity == nil && r.EntityTitle.Valid {
			entity = &EntityInfo{
				Type:       "episode",
				ID:         entityID,
				Title:      r.EntityTitle.String,
				PosterPath: stringFromNullString(r.PosterPath),
			}
		}
		entry := &HistoryEntry{
			SortKey: r.SortKey,
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

	return &HistoryResponse{
		Entity:  entity,
		History: history,
	}, nil
}

func (js *JobService) getEntityTransitionsJob(ctx context.Context, entityID int64) (*HistoryResponse, error) {
	rows, err := js.activityStorage.GetEntityTransitionsJob(ctx, entityID)
	if err != nil {
		return nil, fmt.Errorf("failed to get job transitions: %w", err)
	}

	var entity *EntityInfo
	history := make([]*HistoryEntry, 0, len(rows))

	for _, r := range rows {
		if entity == nil {
			entity = &EntityInfo{
				Type:  "job",
				ID:    entityID,
				Title: r.EntityTitle,
			}
		}
		entry := &HistoryEntry{
			SortKey: r.SortKey,
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

	return &HistoryResponse{
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

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int64(d.Seconds()))
	}
	if d < time.Hour {
		minutes := int64(d.Minutes())
		seconds := int64(d.Seconds()) % 60
		if seconds > 0 {
			return fmt.Sprintf("%dm %ds", minutes, seconds)
		}
		return fmt.Sprintf("%dm", minutes)
	}
	hours := int64(d.Hours())
	minutes := int64(d.Minutes()) % 60
	if minutes > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dh", hours)
}
