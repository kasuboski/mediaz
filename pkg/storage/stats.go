package storage

import (
	"context"

	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
)

type StatisticsStorage interface {
	sqlcdb.Querier
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
