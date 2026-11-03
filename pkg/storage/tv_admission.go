package storage

import "sync"

// LockTVAdmission serializes final TV admission and policy edits in this process.
// Stripes are bounded; unrelated series may briefly share a lock.
var tvAdmissionLocks [128]sync.Mutex

func LockTVAdmission(seriesID int64) func() {
	m := &tvAdmissionLocks[uint64(seriesID)%uint64(len(tvAdmissionLocks))]
	m.Lock()
	return m.Unlock
}
