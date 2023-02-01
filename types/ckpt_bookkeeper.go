package types

import (
	"sync"
)

type CheckpointsBookkeeper struct {
	// checkpoints that have not been reported
	checkpointRecords map[string]*CheckpointRecord

	sync.RWMutex
}

func NewCheckpointsBookkeeper() *CheckpointsBookkeeper {
	records := make(map[string]*CheckpointRecord, 0)
	return &CheckpointsBookkeeper{
		checkpointRecords: records,
	}
}

// Add adds a new checkpoint record into the bookkeeper
// replace with the older one if the checkpoint id exists
func (cb *CheckpointsBookkeeper) Add(cr *CheckpointRecord) {
	cb.Lock()
	defer cb.Unlock()

	id := cr.ID()

	if !cb.has(id) {
		cb.checkpointRecords[id] = cr
	} else {
		// replace with the older one if the checkpoint id exists
		if cb.checkpointRecords[id].FirstSeenBtcHeight > cr.FirstSeenBtcHeight {
			cb.checkpointRecords[id] = cr
		}
	}
}

func (cb *CheckpointsBookkeeper) Remove(id string) {
	cb.Lock()
	defer cb.Unlock()

	delete(cb.checkpointRecords, id)
}

func (cb *CheckpointsBookkeeper) has(id string) bool {
	_, exists := cb.checkpointRecords[id]
	return exists
}

func (cb *CheckpointsBookkeeper) GetAll() []*CheckpointRecord {
	cb.Lock()
	defer cb.Unlock()

	var res []*CheckpointRecord
	for _, r := range cb.checkpointRecords {
		res = append(res, r)
	}

	return res
}
