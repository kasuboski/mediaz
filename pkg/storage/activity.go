package storage

import (
	"context"
	"time"
)

type ActiveMovie struct {
	ID             int64               `json:"id"`
	TMDBID         int64               `json:"tmdbID"`
	Title          string              `json:"title"`
	Year           int                 `json:"year"`
	PosterPath     string              `json:"poster_path,omitempty"`
	State          string              `json:"state"`
	StateSince     time.Time           `json:"stateSince"`
	DownloadClient *DownloadClientInfo `json:"downloadClient"`
	DownloadID     string              `json:"downloadID"`
}

type ActiveSeries struct {
	ID             int64               `json:"id"`
	TMDBID         int64               `json:"tmdbID"`
	Title          string              `json:"title"`
	Year           int                 `json:"year"`
	PosterPath     string              `json:"poster_path,omitempty"`
	State          string              `json:"state"`
	StateSince     time.Time           `json:"stateSince"`
	DownloadClient *DownloadClientInfo `json:"downloadClient"`
	DownloadID     string              `json:"downloadID"`
	SeasonNumber   *int                `json:"seasonNumber,omitempty"`
	EpisodeNumber  *int                `json:"episodeNumber,omitempty"`
}

type ActiveJob struct {
	ID        int64     `json:"id"`
	Type      string    `json:"type"`
	State     string    `json:"state"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type DownloadClientInfo struct {
	ID   int    `json:"id"`
	Host string `json:"host"`
	Port int    `json:"port"`
}

type TimelineEntry struct {
	Date   string        `json:"date"`
	Movies *MovieCounts  `json:"movies,omitempty"`
	Series *SeriesCounts `json:"series,omitempty"`
	Jobs   *JobCounts    `json:"jobs,omitempty"`
}

type MovieCounts struct {
	Downloaded  int `json:"downloaded"`
	Downloading int `json:"downloading"`
}

type SeriesCounts struct {
	Completed   int `json:"completed"`
	Downloading int `json:"downloading"`
}

type JobCounts struct {
	Done  int `json:"done"`
	Error int `json:"error"`
}

type TransitionItem struct {
	ID          int64     `json:"id"`
	EntityType  string    `json:"entityType"`
	EntityID    int64     `json:"entityId"`
	EntityTitle string    `json:"entityTitle"`
	ToState     string    `json:"toState"`
	FromState   *string   `json:"fromState,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

type HistoryEntry struct {
	SortKey   int                 `json:"sortKey"`
	ToState   string              `json:"toState"`
	FromState *string             `json:"fromState,omitempty"`
	CreatedAt time.Time           `json:"createdAt"`
	Metadata  *TransitionMetadata `json:"metadata,omitempty"`
}

type TransitionMetadata struct {
	DownloadClient *DownloadClientInfo `json:"downloadClient,omitempty"`
	DownloadID     string              `json:"downloadID,omitempty"`
}

type TimelineResponse struct {
	Timeline    []*TimelineEntry  `json:"timeline"`
	Transitions []*TransitionItem `json:"transitions"`
	Count       int               `json:"count"`
}

type HistoryResponse struct {
	Entity  *EntityInfo     `json:"entity"`
	History []*HistoryEntry `json:"history"`
}

type EntityInfo struct {
	Type       string `json:"type"`
	ID         int64  `json:"id"`
	Title      string `json:"title"`
	PosterPath string `json:"poster_path,omitempty"`
}

type ActivityStorage interface {
	ListDownloadingMovies(ctx context.Context) ([]*ActiveMovie, error)
	ListDownloadingSeries(ctx context.Context) ([]*ActiveSeries, error)
	ListRunningJobs(ctx context.Context) ([]*ActiveJob, error)
	ListErrorJobs(ctx context.Context, hours int) ([]*ActiveJob, error)
	GetTransitionsByDate(ctx context.Context, startDate, endDate time.Time, offset, limit int) (*TimelineResponse, error)
	CountTransitionsByDate(ctx context.Context, startDate, endDate time.Time) (int, error)
	GetEntityTransitions(ctx context.Context, entityType string, entityID int64) (*HistoryResponse, error)
}
