package storage

import "context"

type Storage interface {
	Init(ctx context.Context, schemas ...string) error
	CreateIndexer(ctx context.Context, name, uri, apiKey string, priority int) (int64, error)
	DeleteIndexer(ctx context.Context, id int64) error
}
