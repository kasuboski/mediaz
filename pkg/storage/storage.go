package storage

import (
	"context"

	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
)

type Storage interface {
	Init(ctx context.Context, schemas ...string) error
	IndexerStorage
	QualityDefinitionStorage
	QualityProfileStorage
}

type IndexerStorage interface {
	CreateIndexer(ctx context.Context, indexer model.Indexer) (int64, error)
	DeleteIndexer(ctx context.Context, id int64) error
	ListIndexers(ctx context.Context) ([]*model.Indexer, error)
}

type QualityDefinitionStorage interface {
	CreateQualityDefinition(ctx context.Context, definition model.QualityDefinition) (int64, error)
	ListQualityDefinitions(ctx context.Context) ([]*model.QualityDefinition, error)
	DeleteQualityDefinition(ctx context.Context, id int64) error
}

type QualityProfileStorage interface {
	GetQualityProfile(ctx context.Context, id int64) (QualityProfile, error)
	ListQualityProfiles(ctx context.Context) ([]QualityProfile, error)
}

type QualityProfile struct {
	ID               int32 `sql:"id,primary_key"`
	Name             string
	Cutoff           int32
	UpgradeAllowed   bool
	ProfileQualityID int32 `sql:"profile_quality_id"`
}
