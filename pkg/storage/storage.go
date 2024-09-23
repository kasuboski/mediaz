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
	Name           string        `json:"name"`
	Items          []QualityItem `json:"items"`
	ID             int32         `sql:"primary_key" json:"id"`
	Cutoff         int32         `json:"cutoff"`
	UpgradeAllowed bool          `json:"upgradeAllowed"`
}

type QualityItem struct {
	ParentID          *int32            `alias:"quality_item.parent_id" json:"parentID"`
	Name              string            `alias:"quality_item.name" json:"name"`
	QualityDefinition QualityDefinition `json:"quality"`
	ID                int32             `alias:"quality_item.id" json:"id"`
	Allowed           bool              `alias:"quality_item.allowed" json:"allowed"`
}

type QualityDefinition struct {
	QualityID     *int32  `alias:"quality_definition.quality_id" json:"-"`
	Name          string  `alias:"quality_definition.name" json:"name"`
	MediaType     string  `alias:"quality_definition.media_type" json:"type"`
	PreferredSize float64 `alias:"quality_definition.preferred_size" json:"preferredSize"`
	MinSize       float64 `alias:"quality_definition.min_size" json:"minSize"`
	MaxSize       float64 `alias:"quality_definition.max_size" json:"maxSize"`
}
