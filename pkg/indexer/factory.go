package indexer

import (
	"errors"
	"fmt"

	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
)

type IndexerSourceFactory struct{}

func NewIndexerSourceFactory() Factory {
	return &IndexerSourceFactory{}
}

func (f *IndexerSourceFactory) NewIndexerSource(config model.IndexerSource) (IndexerSource, error) {
	if !config.Enabled {
		return nil, errors.New("indexer source is disabled")
	}

	switch config.Implementation {
	case "prowlarr":
		return NewProwlarrSource(config)
	default:
		return nil, fmt.Errorf("unsupported indexer source implementation: %s", config.Implementation)
	}
}
