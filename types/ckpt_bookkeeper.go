package types

import (
	"sync"
)

type CheckpointsBookkeeper struct {
	// checkpoints that have not been reported
	checkpointRecords map[CheckpointId]*CheckpointRecord

	sync.RWMutex
}

func NewCheckpointsBookkeeper() *CheckpointsBookkeeper {
	records := make(map[CheckpointId]*CheckpointRecord, 0)
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

	if !cb.Exists(id) {
		cb.checkpointRecords[id] = cr
	} else {
		// replace with the older one if the checkpoint id exists
		if cb.checkpointRecords[id].firstBtcHeight > cr.firstBtcHeight {
			cb.checkpointRecords[id] = cr
		}
	}
}

func (cb *CheckpointsBookkeeper) Remove(id CheckpointId) {
	cb.Lock()
	defer cb.Unlock()

	delete(cb.checkpointRecords, id)
}

func (cb *CheckpointsBookkeeper) Exists(id CheckpointId) bool {
	cb.Lock()
	defer cb.Unlock()

	_, exists := cb.checkpointRecords[id]
	return exists
}

func (cb *CheckpointsBookkeeper) size() int {
	return len(cb.checkpointRecords)
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
