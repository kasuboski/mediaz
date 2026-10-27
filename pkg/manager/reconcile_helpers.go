package manager

import (
	"sync"
	"time"

	sqlcdb "github.com/kasuboski/mediaz/pkg/storage/sqlcdb"
)

// ReconcileSnapshot is a thread safe snapshot of the current reconcile loop state.
type ReconcileSnapshot struct {
	time              time.Time
	downloadProtocols map[string]struct{}
	downloadClients   []sqlcdb.DownloadClient
	indexers          []sqlcdb.Indexer
	indexerIDs        []int64
	mu                sync.Mutex
}

func newReconcileSnapshot(indexers []sqlcdb.Indexer, downloadClients []sqlcdb.DownloadClient) *ReconcileSnapshot {
	ids := make([]int64, 0)
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

func (r *ReconcileSnapshot) GetDownloadClient(id int64) *sqlcdb.DownloadClient {
	dcs := r.GetDownloadClients()

	for i := range dcs {
		if dcs[i].ID == id {
			return &dcs[i]
		}
	}

	return nil
}

func (r *ReconcileSnapshot) GetDownloadClients() []sqlcdb.DownloadClient {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.downloadClients
}

func (r *ReconcileSnapshot) GetProtocols() map[string]struct{} {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.downloadProtocols
}

func (r *ReconcileSnapshot) GetIndexerIDs() []int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.indexerIDs
}

func (r *ReconcileSnapshot) GetIndexers() []sqlcdb.Indexer {
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
