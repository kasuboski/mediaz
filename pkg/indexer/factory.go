package indexer

import (
	"errors"
	"fmt"
	"sync"

	"github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
)

type IndexerSourceFactory struct {
	mu      sync.Mutex
	newznab map[string]*NewznabIndexerSource
}

func NewIndexerSourceFactory() Factory {
	return &IndexerSourceFactory{newznab: make(map[string]*NewznabIndexerSource)}
}

func (f *IndexerSourceFactory) NewIndexerSource(config sqlcdb.IndexerSource) (IndexerSource, error) {
	if !config.Enabled {
		return nil, errors.New("indexer source is disabled")
	}

	switch config.Implementation {
	case "prowlarr":
		return NewProwlarrSource(config)
	case "newznab":
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.newznab == nil {
			f.newznab = make(map[string]*NewznabIndexerSource)
		}
		key := fmt.Sprintf("%s|%s|%s|%d|%s|%s", config.Scheme, config.Host, config.ApiKey.String, config.Port.Int64, config.Name, config.ApiPath.String)
		if source := f.newznab[key]; source != nil {
			return source, nil
		}
		source, err := NewNewznabSource(config)
		if err != nil {
			return nil, err
		}
		if len(f.newznab) >= 64 {
			for k := range f.newznab {
				delete(f.newznab, k)
				break
			}
		}
		f.newznab[key] = source
		return source, nil
	default:
		return nil, fmt.Errorf("unsupported indexer source implementation: %s", config.Implementation)
	}
}
