package storage

import "context"

type Storage interface {
	CreateIndexer(ctx context.Context, name string, priority int) (int64, error)
	DeleteIndexer(ctx context.Context, id int64) error
}
