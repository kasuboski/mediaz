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

func (js *JobService) GetActiveActivity(ctx context.Context, params ActivePaginationParams) (*ActiveActivityResponse, error) {
	log := logger.FromCtx(ctx)
	movieOff, movieLim := params.Movies.CalculateOffsetLimit()
	seriesOff, seriesLim := params.Series.CalculateOffsetLimit()
	jobOff, jobLim := params.Jobs.CalculateOffsetLimit()

	movieRows, err := js.activityStorage.ListDownloadingMoviesPage(ctx, sqlcdb.ListDownloadingMoviesPageParams{Offs: int64(movieOff), Lim: int64(movieLim)})
	if err != nil {
		log.Error("failed to list downloading movies", zap.Error(err))
		return nil, err
	}
	movieCount, err := js.activityStorage.CountDownloadingMovies(ctx)
	if err != nil {
		return nil, err
	}
	seriesRows, err := js.activityStorage.ListDownloadingSeriesPage(ctx, sqlcdb.ListDownloadingSeriesPageParams{Offs: int64(seriesOff), Lim: int64(seriesLim)})
	if err != nil {
		log.Error("failed to list downloading series", zap.Error(err))
		return nil, err
	}
	seriesCount, err := js.activityStorage.CountDownloadingSeries(ctx)
	if err != nil {
		return nil, err
	}
	jobRows, err := js.activityStorage.ListRunningJobsPage(ctx, sqlcdb.ListRunningJobsPageParams{Offs: int64(jobOff), Lim: int64(jobLim)})
	if err != nil {
		log.Error("failed to list running jobs", zap.Error(err))
		return nil, err
	}
	jobCount, err := js.activityStorage.CountRunningJobs(ctx)
	if err != nil {
		return nil, err
	}

	movies := make([]*ActiveMovie, 0, len(movieRows))
	for _, r := range movieRows {
		movies = append(movies, transformActiveMovie(r))
	}
	series := make([]*ActiveSeries, 0, len(seriesRows))
	for _, r := range seriesRows {
		series = append(series, transformActiveSeriesRow(r))
	}
	jobs := make([]*ActiveJob, 0, len(jobRows))
	for _, r := range jobRows {
		jobs = append(jobs, transformActiveJobRow(r))
	}
	return &ActiveActivityResponse{Movies: movies, Series: series, Jobs: jobs,
		Counts:     ActiveActivityCounts{Movies: int(movieCount), Series: int(seriesCount), Jobs: int(jobCount), Total: int(movieCount + seriesCount + jobCount)},
		Pagination: ActiveActivityPagination{Movies: params.Movies.BuildMeta(int(movieCount)), Series: params.Series.BuildMeta(int(seriesCount)), Jobs: params.Jobs.BuildMeta(int(jobCount))}}, nil
}

func transformActiveMovie(r sqlcdb.ListDownloadingMoviesPageRow) *ActiveMovie {
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
	return m
}

func transformActiveSeriesRow(r sqlcdb.ListDownloadingSeriesPageRow) *ActiveSeries {
	s := &ActiveSeries{ID: r.ID, State: r.ToState}
	if r.CreatedAt.Valid {
		s.StateSince = r.CreatedAt.Time
	}
	s.Duration = formatDuration(time.Since(s.StateSince))
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
	return s
}

func transformActiveJobRow(r sqlcdb.ListRunningJobsPageRow) *ActiveJob {
	j := &ActiveJob{ID: r.ID, Type: r.Type, State: r.ToState}
	if r.CreatedAt.Valid {
		j.CreatedAt = r.CreatedAt.Time
	}
	if r.UpdatedAt.Valid {
		j.UpdatedAt = r.UpdatedAt.Time
	}
	j.Duration = formatDuration(time.Since(j.UpdatedAt))
	return j
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

func (js *JobService) GetRecentFailures(ctx context.Context, hours int, params pagination.Params) (*FailuresResponse, error) {
	if hours <= 0 || int64(hours) > int64((1<<63-1)/int64(time.Hour)) {
		return nil, fmt.Errorf("hours must be positive and representable")
	}
	now := time.Now().UTC()
	cutoff := now.Add(-time.Duration(hours) * time.Hour)
	cutoffParam := sql.NullTime{Time: cutoff, Valid: true}
	offset, limit := params.CalculateOffsetLimit()
	rows, err := js.activityStorage.ListErrorJobsPage(ctx, sqlcdb.ListErrorJobsPageParams{CutoffDatetime: cutoffParam, Offs: int64(offset), Lim: int64(limit)})
	if err != nil {
		logger.FromCtx(ctx).Error("failed to list error jobs", zap.Error(err))
		return nil, err
	}
	count, err := js.activityStorage.CountErrorJobs(ctx, cutoffParam)
	if err != nil {
		return nil, err
	}
	failures := make([]*FailureItem, 0, len(rows))
	for _, job := range rows {
		if !job.UpdatedAt.Valid {
			continue
		}
		failures = append(failures, &FailureItem{Type: "job", ID: job.ID, Title: job.Type, State: job.ToState, FailedAt: job.UpdatedAt.Time, Retryable: true})
	}
	return &FailuresResponse{Failures: failures, Pagination: params.BuildMeta(int(count))}, nil
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

func (js *JobService) GetEntityTransitionHistory(ctx context.Context, entityType string, entityID int64, params pagination.Params) (*HistoryResponse, error) {
	offset, limit := params.CalculateOffsetLimit()
	response, err := js.getEntityTransitions(ctx, entityType, entityID, offset, limit)
	if err != nil {
		return nil, err
	}
	if response.History == nil {
		response.History = []*HistoryEntry{}
	}
	if response.Entity == nil && response.Pagination.TotalItems > 0 && len(response.History) == 0 {
		header, err := js.getEntityTransitions(ctx, entityType, entityID, 0, 1)
		if err != nil {
			return nil, err
		}
		if len(header.History) > 0 {
			response.Entity = header.Entity
		}
	}
	return response, nil
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

	// Fetch only the globally ordered page; metadata joins in SQL preserve eligibility.
	pageRows, err := js.activityStorage.GetActivityTimelinePage(ctx, sqlcdb.GetActivityTimelinePageParams{
		Start: sd, End: ed, Offs: int64(offset), Lim: int64(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get activity timeline page: %w", err)
	}
	transitions := make([]*TransitionItem, 0, len(pageRows))
	for _, row := range pageRows {
		transitions = append(transitions, convertTransitionItemRow(row.ID, row.EntityType, row.EntityID, row.EntityTitle, row.ToState, row.FromState, row.CreatedAt))
	}

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

func (js *JobService) getEntityTransitions(ctx context.Context, entityType string, entityID int64, offset, limit int) (*HistoryResponse, error) {
	if offset < 0 || limit <= 0 {
		return nil, fmt.Errorf("offset must be nonnegative and limit must be positive")
	}
	var rows []*HistoryEntry
	var entity *EntityInfo
	var count int64
	var err error
	// Keep the metadata shape and ordering supplied by the legacy transforms while
	// page-specific SQL computes durations against the complete history.
	switch entityType {
	case "movie":
		r, e := js.activityStorage.GetMovieHistoryPage(ctx, sqlcdb.GetMovieHistoryPageParams{EntityID: entityID, Offs: int64(offset), Lim: int64(limit)})
		err = e
		if e == nil {
			count, err = js.activityStorage.CountMovieHistory(ctx, entityID)
		}
		if err == nil {
			for _, x := range r {
				if entity == nil && x.EntityTitle.Valid {
					entity = &EntityInfo{Type: entityType, ID: entityID, Title: x.EntityTitle.String, PosterPath: stringFromNullString(x.PosterPath)}
				}
				entry := &HistoryEntry{SortKey: x.SortKey, ToState: x.ToState}
				if x.FromState.Valid {
					entry.FromState = &x.FromState.String
				}
				if x.CreatedAt.Valid {
					entry.CreatedAt = x.CreatedAt.Time
				}
				end := time.Now().UTC()
				if t, ok := parseActivityTimestamp(x.NextCreatedAt); ok {
					end = t
				}
				entry.Duration = formatDuration(end.Sub(entry.CreatedAt))
				if x.Metadata != "" {
					entry.Metadata = &TransitionMetadata{}
					if e := json.Unmarshal([]byte(x.Metadata), entry.Metadata); e != nil {
						return nil, e
					}
				}
				rows = append(rows, entry)
			}
		}
	case "series", "season", "episode", "job":
		// Typed row sets share the same ordering and nullable fields. Dispatch the
		// database page/count operations here; transforms retain entity metadata.
		switch entityType {
		case "series":
			return js.getPagedSeriesHistory(ctx, entityID, offset, limit)
		case "season":
			return js.getPagedSeasonHistory(ctx, entityID, offset, limit)
		case "episode":
			return js.getPagedEpisodeHistory(ctx, entityID, offset, limit)
		case "job":
			return js.getPagedJobHistory(ctx, entityID, offset, limit)
		}
	default:
		return nil, fmt.Errorf("unsupported entity type: %s", entityType)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get %s history: %w", entityType, err)
	}
	return &HistoryResponse{Entity: entity, History: rows, Pagination: pagination.Params{Page: offset/limit + 1, PageSize: limit}.BuildMeta(int(count))}, nil
}

func parseActivityTimestamp(v interface{}) (time.Time, bool) {
	switch x := v.(type) {
	case time.Time:
		return x.UTC(), true
	case string:
		for _, f := range []string{time.RFC3339Nano, "2006-01-02 15:04:05.999999999", "2006-01-02 15:04:05"} {
			if t, e := time.Parse(f, x); e == nil {
				return t.UTC(), true
			}
		}
	case []byte:
		return parseActivityTimestamp(string(x))
	}
	return time.Time{}, false
}

func (js *JobService) getPagedSeriesHistory(ctx context.Context, id int64, off, lim int) (*HistoryResponse, error) {
	r, e := js.activityStorage.GetSeriesHistoryPage(ctx, sqlcdb.GetSeriesHistoryPageParams{EntityID: id, Offs: int64(off), Lim: int64(lim)})
	if e != nil {
		return nil, e
	}
	c, e := js.activityStorage.CountSeriesHistory(ctx, id)
	if e != nil {
		return nil, e
	}
	h := make([]*HistoryEntry, 0, len(r))
	var entity *EntityInfo
	now := time.Now().UTC()
	for _, x := range r {
		if entity == nil && x.EntityTitle.Valid {
			entity = &EntityInfo{Type: "series", ID: id, Title: x.EntityTitle.String, PosterPath: stringFromNullString(x.PosterPath)}
		}
		a := &HistoryEntry{SortKey: x.SortKey, ToState: x.ToState}
		if x.FromState.Valid {
			a.FromState = &x.FromState.String
		}
		if x.CreatedAt.Valid {
			a.CreatedAt = x.CreatedAt.Time
		}
		end := now
		if t, ok := parseActivityTimestamp(x.NextCreatedAt); ok {
			end = t
		}
		a.Duration = formatDuration(end.Sub(a.CreatedAt))
		h = append(h, a)
	}
	return &HistoryResponse{Entity: entity, History: h, Pagination: pagination.Params{Page: off/lim + 1, PageSize: lim}.BuildMeta(int(c))}, nil
}
func (js *JobService) getPagedSeasonHistory(ctx context.Context, id int64, off, lim int) (*HistoryResponse, error) {
	r, e := js.activityStorage.GetSeasonHistoryPage(ctx, sqlcdb.GetSeasonHistoryPageParams{EntityID: id, Offs: int64(off), Lim: int64(lim)})
	if e != nil {
		return nil, e
	}
	c, e := js.activityStorage.CountSeasonHistory(ctx, id)
	if e != nil {
		return nil, e
	}
	h := make([]*HistoryEntry, 0, len(r))
	var entity *EntityInfo
	now := time.Now().UTC()
	for _, x := range r {
		if entity == nil && x.EntityTitle.Valid {
			entity = &EntityInfo{Type: "season", ID: id, Title: x.EntityTitle.String, PosterPath: stringFromNullString(x.PosterPath)}
		}
		a := &HistoryEntry{SortKey: x.SortKey, ToState: x.ToState}
		if x.FromState.Valid {
			a.FromState = &x.FromState.String
		}
		if x.CreatedAt.Valid {
			a.CreatedAt = x.CreatedAt.Time
		}
		end := now
		if t, ok := parseActivityTimestamp(x.NextCreatedAt); ok {
			end = t
		}
		a.Duration = formatDuration(end.Sub(a.CreatedAt))
		if x.Metadata != "" {
			a.Metadata = &TransitionMetadata{}
			if e := json.Unmarshal([]byte(x.Metadata), a.Metadata); e != nil {
				return nil, e
			}
		}
		h = append(h, a)
	}
	return &HistoryResponse{Entity: entity, History: h, Pagination: pagination.Params{Page: off/lim + 1, PageSize: lim}.BuildMeta(int(c))}, nil
}
func (js *JobService) getPagedEpisodeHistory(ctx context.Context, id int64, off, lim int) (*HistoryResponse, error) {
	r, e := js.activityStorage.GetEpisodeHistoryPage(ctx, sqlcdb.GetEpisodeHistoryPageParams{EntityID: id, Offs: int64(off), Lim: int64(lim)})
	if e != nil {
		return nil, e
	}
	c, e := js.activityStorage.CountEpisodeHistory(ctx, id)
	if e != nil {
		return nil, e
	}
	h := make([]*HistoryEntry, 0, len(r))
	var entity *EntityInfo
	now := time.Now().UTC()
	for _, x := range r {
		if entity == nil && x.EntityTitle.Valid {
			entity = &EntityInfo{Type: "episode", ID: id, Title: x.EntityTitle.String, PosterPath: stringFromNullString(x.PosterPath)}
		}
		a := &HistoryEntry{SortKey: x.SortKey, ToState: x.ToState}
		if x.FromState.Valid {
			a.FromState = &x.FromState.String
		}
		if x.CreatedAt.Valid {
			a.CreatedAt = x.CreatedAt.Time
		}
		end := now
		if t, ok := parseActivityTimestamp(x.NextCreatedAt); ok {
			end = t
		}
		a.Duration = formatDuration(end.Sub(a.CreatedAt))
		h = append(h, a)
	}
	return &HistoryResponse{Entity: entity, History: h, Pagination: pagination.Params{Page: off/lim + 1, PageSize: lim}.BuildMeta(int(c))}, nil
}
func (js *JobService) getPagedJobHistory(ctx context.Context, id int64, off, lim int) (*HistoryResponse, error) {
	r, e := js.activityStorage.GetJobHistoryPage(ctx, sqlcdb.GetJobHistoryPageParams{EntityID: id, Offs: int64(off), Lim: int64(lim)})
	if e != nil {
		return nil, e
	}
	c, e := js.activityStorage.CountJobHistory(ctx, id)
	if e != nil {
		return nil, e
	}
	h := make([]*HistoryEntry, 0, len(r))
	var entity *EntityInfo
	now := time.Now().UTC()
	for _, x := range r {
		if entity == nil {
			entity = &EntityInfo{Type: "job", ID: id, Title: x.EntityTitle}
		}
		a := &HistoryEntry{SortKey: x.SortKey, ToState: x.ToState}
		if x.FromState.Valid {
			a.FromState = &x.FromState.String
		}
		if x.CreatedAt.Valid {
			a.CreatedAt = x.CreatedAt.Time
		}
		end := now
		if t, ok := parseActivityTimestamp(x.NextCreatedAt); ok {
			end = t
		}
		a.Duration = formatDuration(end.Sub(a.CreatedAt))
		h = append(h, a)
	}
	return &HistoryResponse{Entity: entity, History: h, Pagination: pagination.Params{Page: off/lim + 1, PageSize: lim}.BuildMeta(int(c))}, nil
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
