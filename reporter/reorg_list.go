package reporter

import (
	"sync"

	sdkmath "cosmossdk.io/math"
	btclightclienttypes "github.com/babylonchain/babylon/x/btclightclient/types"
	"github.com/btcsuite/btcd/wire"
)

type removedBlock struct {
	height uint64
	header *wire.BlockHeader
}

// Help data structure to keep track of removed blocks.
// NOTE: This is not generic data structure, and must be used with conjunction with
// reporter and btc cache
type reorgList struct {
	sync.Mutex
	workOfRemovedBlocks sdkmath.Uint
	removedBlocks       []*removedBlock
}

func newReorgList() *reorgList {
	return &reorgList{
		removedBlocks:       []*removedBlock{},
		workOfRemovedBlocks: sdkmath.ZeroUint(),
	}
}

// addRemovedBlock add currently removed block to the end of the list. Re-orgs
// are started from the tip of the chain and go backwards, this means
// that oldest removed block is at the end of the list.
func (r *reorgList) addRemovedBlock(
	height uint64,
	header *wire.BlockHeader) {
	headerWork := btclightclienttypes.CalcHeaderWork(header)
	r.Lock()
	defer r.Unlock()

	newWork := btclightclienttypes.CumulativeWork(headerWork, r.workOfRemovedBlocks)
	r.removedBlocks = append(r.removedBlocks, &removedBlock{height, header})
	r.workOfRemovedBlocks = newWork
}

func (r *reorgList) getLastRemovedBlock() *removedBlock {
	r.Lock()
	defer r.Unlock()
	if len(r.removedBlocks) == 0 {
		return nil
	}

	return r.removedBlocks[len(r.removedBlocks)-1]
}

func (r *reorgList) clear() {
	r.Lock()
	defer r.Unlock()

	r.removedBlocks = []*removedBlock{}
	r.workOfRemovedBlocks = sdkmath.ZeroUint()
}

func (r *reorgList) size() int {
	r.Lock()
	defer r.Unlock()

	return len(r.removedBlocks)
}

func (r *reorgList) removedBranchWork() sdkmath.Uint {
	r.Lock()
	defer r.Unlock()
	return r.workOfRemovedBlocks
}
