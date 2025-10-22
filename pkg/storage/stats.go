package storage

import (
	"context"
)

// StatisticsStorage interface for optimized statistics queries
type StatisticsStorage interface {
	GetMovieStatsByState(ctx context.Context) ([]MovieStatsByState, error)
	GetTVStatsByState(ctx context.Context) ([]TVStatsByState, error)
	GetLibraryStats(ctx context.Context) (*LibraryStats, error)
}

type MovieStatsByState struct {
	State string `json:"state"`
	Count int    `json:"count"`
}

type TVStatsByState struct {
	State string `json:"state"`
	Count int    `json:"count"`
}

// Statistics types
type MovieStats struct {
	Total   int                `json:"total"`
	ByState map[MovieState]int `json:"byState"`
}

type TVStats struct {
	Total   int                 `json:"total"`
	ByState map[SeriesState]int `json:"byState"`
}

type LibraryStats struct {
	Movies MovieStats `json:"movies"`
	TV     TVStats    `json:"tv"`
}
