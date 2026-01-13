package manager

import "time"

type ActiveActivityResponse struct {
	Movies []*ActiveMovie  `json:"movies"`
	Series []*ActiveSeries `json:"series"`
	Jobs   []*ActiveJob    `json:"jobs"`
}

type ActiveMovie struct {
	ID             int64               `json:"id"`
	TMDBID         int64               `json:"tmdbID"`
	Title          string              `json:"title"`
	Year           int                 `json:"year"`
	PosterPath     string              `json:"poster_path,omitempty"`
	State          string              `json:"state"`
	StateSince     time.Time           `json:"stateSince"`
	Duration       string              `json:"duration"`
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
	Duration       string              `json:"duration"`
	DownloadClient *DownloadClientInfo `json:"downloadClient"`
	DownloadID     string              `json:"downloadID"`
	CurrentEpisode *EpisodeInfo        `json:"currentEpisode,omitempty"`
}

type ActiveJob struct {
	ID        int64     `json:"id"`
	Type      string    `json:"type"`
	State     string    `json:"state"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Duration  string    `json:"duration"`
}

type DownloadClientInfo struct {
	ID   int    `json:"id"`
	Host string `json:"host"`
	Port int    `json:"port"`
}

type EpisodeInfo struct {
	SeasonNumber  int `json:"seasonNumber"`
	EpisodeNumber int `json:"episodeNumber"`
}

type FailuresResponse struct {
	Failures []*FailureItem `json:"failures"`
}

type FailureItem struct {
	Type      string    `json:"type"`
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	Subtitle  string    `json:"subtitle"`
	State     string    `json:"state"`
	FailedAt  time.Time `json:"failedAt"`
	Error     string    `json:"error"`
	Retryable bool      `json:"retryable"`
}

type TimelineResponse struct {
	Timeline    []*TimelineEntry  `json:"timeline"`
	Transitions []*TransitionItem `json:"transitions"`
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

type HistoryEntry struct {
	SortKey   int                 `json:"sortKey"`
	ToState   string              `json:"toState"`
	FromState *string             `json:"fromState,omitempty"`
	CreatedAt time.Time           `json:"createdAt"`
	Duration  string              `json:"duration"`
	Metadata  *TransitionMetadata `json:"metadata,omitempty"`
}

type TransitionMetadata struct {
	DownloadClient *DownloadClientInfo `json:"downloadClient,omitempty"`
	DownloadID     string              `json:"downloadID,omitempty"`
}
