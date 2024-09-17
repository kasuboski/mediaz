package storage

import (
	"context"

	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
)

type Storage interface {
	Init(ctx context.Context, schemas ...string) error
	IndexerStorage
	QualityDefinitionStorage
	// QualityProfileStorage
}

type IndexerStorage interface {
	CreateIndexer(ctx context.Context, indexer model.Indexers) (int64, error)
	DeleteIndexer(ctx context.Context, id int64) error
	ListIndexers(ctx context.Context) ([]*model.Indexers, error)
}

type QualityDefinitionStorage interface {
	CreateQualityDefinition(ctx context.Context, definition model.QualityDefinitions) (int64, error)
	ListQualityDefinitions(ctx context.Context) ([]*model.QualityDefinitions, error)
	DeleteQualityDefinition(ctx context.Context, id int64) error
}
