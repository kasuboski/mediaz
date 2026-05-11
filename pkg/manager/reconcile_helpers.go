package manager

import (
	"sync"
	"time"

	"github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
)

// ReconcileSnapshot is a thread safe snapshot of the current reconcile loop state.
type ReconcileSnapshot struct {
	time              time.Time
	downloadProtocols map[string]struct{}
	downloadClients   []*model.DownloadClient
	indexers          []model.Indexer
	indexerIDs        []int32
	mu                sync.Mutex
}

func newReconcileSnapshot(indexers []model.Indexer, downloadClients []*model.DownloadClient) *ReconcileSnapshot {
	ids := make([]int32, 0)
	for i := range indexers {
		ids = append(ids, indexers[i].ID)
	}

	protocols := availableProtocols(downloadClients)

	return &ReconcileSnapshot{
		downloadClients:   downloadClients,
		downloadProtocols: protocols,
		indexerIDs:        ids,
		indexers:          indexers,
		mu:                sync.Mutex{},
		time:              now(),
	}
}

func (r *ReconcileSnapshot) GetDownloadClient(id int32) *model.DownloadClient {
	dcs := r.GetDownloadClients()

	for _, dc := range dcs {
		if dc.ID == id {
			return dc
		}
	}

	return nil
}

func (r *ReconcileSnapshot) GetDownloadClients() []*model.DownloadClient {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.downloadClients
}

func (r *ReconcileSnapshot) GetProtocols() map[string]struct{} {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.downloadProtocols
}

func (r *ReconcileSnapshot) GetIndexerIDs() []int32 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.indexerIDs
}

func (r *ReconcileSnapshot) GetIndexers() []model.Indexer {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.indexers
}

// now returns the current time. Extracted for testability.
func now() time.Time {
	return time.Now()
}

// isReleased returns true if the release date is set and in the past.
func isReleased(now time.Time, releaseDate *time.Time) bool {
	if releaseDate == nil {
		return false
	}
	if releaseDate.IsZero() {
		return false
	}
	return now.After(*releaseDate)
}
