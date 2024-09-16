package storage

import (
	"context"
	"os"

	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/model"
)

type Storage interface {
	Init(ctx context.Context, schemas ...string) error
	IndexerStorage
}

type IndexerStorage interface {
	CreateIndexer(ctx context.Context, indexer model.Indexers) (int64, error)
	DeleteIndexer(ctx context.Context, id int64) error
	ListIndexers(ctx context.Context) ([]*model.Indexers, error)
}

func ReadSchemaFiles(files ...string) ([]string, error) {
	var schemas []string
	for _, f := range files {
		f, err := os.ReadFile(f)
		if err != nil {
			return schemas, err
		}

		schemas = append(schemas, string(f))
	}

	return schemas, nil
}
