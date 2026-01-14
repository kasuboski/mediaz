package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/kasuboski/mediaz/pkg/logger"
	"github.com/kasuboski/mediaz/pkg/pagination"
	"github.com/kasuboski/mediaz/pkg/storage"
	"go.uber.org/zap"
)

type MovieMetadataImages struct {
	PosterPath string `json:"poster_path,omitempty"`
}

func (m MediaManager) GetActiveActivity(ctx context.Context) (*ActiveActivityResponse, error) {
	log := logger.FromCtx(ctx)

	movies, err := m.storage.ListDownloadingMovies(ctx)
	if err != nil {
		log.Error("failed to list downloading movies", zap.Error(err))
		return nil, err
	}

	series, err := m.storage.ListDownloadingSeries(ctx)
	if err != nil {
		log.Error("failed to list downloading series", zap.Error(err))
		return nil, err
	}

	jobs, err := m.storage.ListRunningJobs(ctx)
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

func transformActiveMovies(movies []*storage.ActiveMovie) []*ActiveMovie {
	result := make([]*ActiveMovie, len(movies))
	for i, m := range movies {
		duration := formatDuration(time.Since(m.StateSince))
		posterPath := ""
		if m.PosterPath != "" {
			var images MovieMetadataImages
			if err := json.Unmarshal([]byte(m.PosterPath), &images); err == nil && images.PosterPath != "" {
				posterPath = images.PosterPath
			}
		}
		result[i] = &ActiveMovie{
			ID:             m.ID,
			TMDBID:         m.TMDBID,
			Title:          m.Title,
			Year:           m.Year,
			PosterPath:     posterPath,
			State:          m.State,
			StateSince:     m.StateSince,
			Duration:       duration,
			DownloadClient: transformDownloadClient(m.DownloadClient),
			DownloadID:     m.DownloadID,
		}
	}
	return result
}

func transformActiveSeries(series []*storage.ActiveSeries) []*ActiveSeries {
	result := make([]*ActiveSeries, len(series))
	for i, s := range series {
		duration := formatDuration(time.Since(s.StateSince))
		posterPath := ""
		if s.PosterPath != "" {
			posterPath = s.PosterPath
		}
		seriesItem := &ActiveSeries{
			ID:             s.ID,
			TMDBID:         s.TMDBID,
			Title:          s.Title,
			Year:           s.Year,
			PosterPath:     posterPath,
			State:          s.State,
			StateSince:     s.StateSince,
			Duration:       duration,
			DownloadClient: transformDownloadClient(s.DownloadClient),
			DownloadID:     s.DownloadID,
		}
		if s.SeasonNumber != nil && s.EpisodeNumber != nil {
			seriesItem.CurrentEpisode = &EpisodeInfo{
				SeasonNumber:  *s.SeasonNumber,
				EpisodeNumber: *s.EpisodeNumber,
			}
		}
		result[i] = seriesItem
	}
	return result
}

func transformActiveJobs(jobs []*storage.ActiveJob) []*ActiveJob {
	result := make([]*ActiveJob, len(jobs))
	for i, j := range jobs {
		duration := formatDuration(time.Since(j.UpdatedAt))
		result[i] = &ActiveJob{
			ID:        j.ID,
			Type:      j.Type,
			State:     j.State,
			CreatedAt: j.CreatedAt,
			UpdatedAt: j.UpdatedAt,
			Duration:  duration,
		}
	}
	return result
}

func transformDownloadClient(dc *storage.DownloadClientInfo) *DownloadClientInfo {
	if dc == nil {
		return nil
	}
	return &DownloadClientInfo{
		ID:   dc.ID,
		Host: dc.Host,
		Port: dc.Port,
	}
}

func (m MediaManager) GetRecentFailures(ctx context.Context, hours int) (*FailuresResponse, error) {
	log := logger.FromCtx(ctx)

	jobs, err := m.storage.ListErrorJobs(ctx)
	if err != nil {
		log.Error("failed to list error jobs", zap.Error(err))
		return nil, err
	}

	cutoff := time.Now().Add(-time.Duration(hours) * time.Hour)

	failures := make([]*FailureItem, 0, len(jobs))
	for _, job := range jobs {
		if job.UpdatedAt.After(cutoff) {
			failures = append(failures, &FailureItem{
				Type:      "job",
				ID:        job.ID,
				Title:     job.Type,
				Subtitle:  "",
				State:     job.State,
				FailedAt:  job.UpdatedAt,
				Error:     "",
				Retryable: true,
			})
		}
	}

	return &FailuresResponse{
		Failures: failures,
	}, nil
}

func (m MediaManager) GetActivityTimeline(ctx context.Context, days int, params pagination.Params) (*TimelineResponse, error) {
	log := logger.FromCtx(ctx)

	startDate := time.Now().AddDate(0, 0, -days)
	endDate := time.Now()

	offset, limit := params.CalculateOffsetLimit()

	storageResp, err := m.storage.GetTransitionsByDate(ctx, startDate, endDate, offset, limit)
	if err != nil {
		log.Error("failed to get transitions by date", zap.Error(err))
		return nil, err
	}

	timeline := make([]*TimelineEntry, len(storageResp.Timeline))
	for i, entry := range storageResp.Timeline {
		timeline[i] = &TimelineEntry{
			Date:   entry.Date,
			Movies: transformMovieCounts(entry.Movies),
			Series: transformSeriesCounts(entry.Series),
			Jobs:   transformJobCounts(entry.Jobs),
		}
	}

	transitions := make([]*TransitionItem, len(storageResp.Transitions))
	for i, t := range storageResp.Transitions {
		transitions[i] = &TransitionItem{
			ID:          t.ID,
			EntityType:  t.EntityType,
			EntityID:    t.EntityID,
			EntityTitle: t.EntityTitle,
			ToState:     t.ToState,
			FromState:   t.FromState,
			CreatedAt:   t.CreatedAt,
		}
	}

	return &TimelineResponse{
		Timeline:    timeline,
		Transitions: transitions,
		Count:       storageResp.Count,
	}, nil
}

func transformMovieCounts(counts *storage.MovieCounts) *MovieCounts {
	if counts == nil {
		return nil
	}
	return &MovieCounts{
		Downloaded:  counts.Downloaded,
		Downloading: counts.Downloading,
	}
}

func transformSeriesCounts(counts *storage.SeriesCounts) *SeriesCounts {
	if counts == nil {
		return nil
	}
	return &SeriesCounts{
		Completed:   counts.Completed,
		Downloading: counts.Downloading,
	}
}

func transformJobCounts(counts *storage.JobCounts) *JobCounts {
	if counts == nil {
		return nil
	}
	return &JobCounts{
		Done:  counts.Done,
		Error: counts.Error,
	}
}

func (m MediaManager) GetEntityTransitionHistory(ctx context.Context, entityType string, entityID int64) (*HistoryResponse, error) {
	log := logger.FromCtx(ctx)

	storageResp, err := m.storage.GetEntityTransitions(ctx, entityType, entityID)
	if err != nil {
		log.Error("failed to get entity transitions", zap.Error(err), zap.String("entityType", entityType), zap.Int64("entityID", entityID))
		return nil, err
	}

	var entityInfo *EntityInfo
	if storageResp.Entity != nil {
		entityInfo = &EntityInfo{
			Type:       storageResp.Entity.Type,
			ID:         storageResp.Entity.ID,
			Title:      storageResp.Entity.Title,
			PosterPath: storageResp.Entity.PosterPath,
		}
	}

	history := calculateDurations(storageResp.History)

	return &HistoryResponse{
		Entity:  entityInfo,
		History: history,
	}, nil
}

func calculateDurations(entries []*storage.HistoryEntry) []*HistoryEntry {
	history := make([]*HistoryEntry, len(entries))
	for i, entry := range entries {
		var duration string
		if i < len(entries)-1 {
			duration = formatDuration(entries[i+1].CreatedAt.Sub(entry.CreatedAt))
		} else {
			duration = formatDuration(time.Since(entry.CreatedAt))
		}

		var metadata *TransitionMetadata
		if entry.Metadata != nil {
			metadata = &TransitionMetadata{
				DownloadClient: nil,
				DownloadID:     entry.Metadata.DownloadID,
			}
			if entry.Metadata.DownloadClient != nil {
				metadata.DownloadClient = &DownloadClientInfo{
					ID:   entry.Metadata.DownloadClient.ID,
					Host: entry.Metadata.DownloadClient.Host,
					Port: entry.Metadata.DownloadClient.Port,
				}
			}
		}

		history[i] = &HistoryEntry{
			SortKey:   entry.SortKey,
			ToState:   entry.ToState,
			FromState: entry.FromState,
			CreatedAt: entry.CreatedAt,
			Duration:  duration,
			Metadata:  metadata,
		}
	}
	return history
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
