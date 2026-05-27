package storage

import (
	"context"
)

type StatisticsStorage interface {
	GetLibraryStats(ctx context.Context) (*LibraryStats, error)
}

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
