package types

import (
	"fmt"
	"sort"
	"sync"
)

type BTCCache struct {
	blocks     []*IndexedBlock
	maxEntries uint64

	sync.RWMutex
}

func NewBTCCache(maxEntries uint64) (*BTCCache, error) {
	// if maxEntries is 0, it means that the cache is disabled
	if maxEntries == 0 {
		return nil, ErrInvalidMaxEntries
	}

	return &BTCCache{
		blocks:     make([]*IndexedBlock, 0, maxEntries),
		maxEntries: maxEntries,
	}, nil
}

// Init initializes the cache with the given blocks. Input blocks should be sorted by height. Thread-safe.
func (b *BTCCache) Init(ibs []*IndexedBlock) error {
	b.Lock()
	defer b.Unlock()

	if len(ibs) > int(b.maxEntries) {
		return ErrTooManyEntries
	}

	// check if the blocks are sorted by height
	if sortedByHeight := sort.SliceIsSorted(ibs, func(i, j int) bool {
		return ibs[i].Height < ibs[j].Height
	}); !sortedByHeight {
		return ErrorUnsortedBlocks
	}

	for _, ib := range ibs {
		b.add(ib)
	}

	return nil
}

// Add adds a new block to the cache. Thread-safe.
func (b *BTCCache) Add(ib *IndexedBlock) {
	b.Lock()
	defer b.Unlock()

	b.add(ib)
}

// Thread-unsafe version of Add
func (b *BTCCache) add(ib *IndexedBlock) {
	if b.size() > b.maxEntries {
		panic(ErrTooManyEntries)
	}
	if b.size() == b.maxEntries {
		// dereference the 0-th block to ensure it will be garbage-collected
		// see https://stackoverflow.com/questions/55045402/memory-leak-in-golang-slice
		b.blocks[0] = nil
		b.blocks = b.blocks[1:]
	}

	b.blocks = append(b.blocks, ib)
}

func (b *BTCCache) First() *IndexedBlock {
	b.RLock()
	defer b.RUnlock()

	if b.size() == 0 {
		return nil
	}

	return b.blocks[0]
}

func (b *BTCCache) Tip() *IndexedBlock {
	b.RLock()
	defer b.RUnlock()

	if b.size() == 0 {
		return nil
	}

	return b.blocks[len(b.blocks)-1]
}

// RemoveLast deletes the last block in cache
func (b *BTCCache) RemoveLast() error {
	b.Lock()
	defer b.Unlock()

	if b.size() == 0 {
		return ErrEmptyCache
	}

	// dereference the last block to ensure it will be garbage-collected
	b.blocks[len(b.blocks)-1] = nil
	b.blocks = b.blocks[:len(b.blocks)-1]
	return nil
}

// RemoveAll deletes all the blocks in cache
func (b *BTCCache) RemoveAll() {
	b.Lock()
	defer b.Unlock()

	b.blocks = []*IndexedBlock{}
}

// Size returns the size of the cache. Thread-safe.
func (b *BTCCache) Size() uint64 {
	b.RLock()
	defer b.RUnlock()

	return b.size()
}

// thread-unsafe version of Size
func (b *BTCCache) size() uint64 {
	return uint64(len(b.blocks))
}

// GetLastBlocks returns list of blocks between the given stopHeight and the tip of the chain in cache
func (b *BTCCache) GetLastBlocks(stopHeight uint64) ([]*IndexedBlock, error) {
	b.RLock()
	defer b.RUnlock()

	firstHeight := b.blocks[0].Height
	lastHeight := b.blocks[len(b.blocks)-1].Height
	if int32(stopHeight) < firstHeight || lastHeight < int32(stopHeight) {
		return []*IndexedBlock{}, fmt.Errorf("the given stopHeight %d is out of range [%d, %d] of BTC cache", stopHeight, firstHeight, lastHeight)
	}

	var j int
	for i := len(b.blocks) - 1; i >= 0; i-- {
		if b.blocks[i].Height == int32(stopHeight) {
			j = i
			break
		}
	}

	return b.blocks[j:], nil
}

// GetAllBlocks returns list of all blocks in cache
func (b *BTCCache) GetAllBlocks() []*IndexedBlock {
	b.RLock()
	defer b.RUnlock()

	return b.blocks
}

// TrimConfirmedBlocks keeps the last <=k blocks in the cache and returns the rest in the same order
// the returned blocks are considered confirmed
func (b *BTCCache) TrimConfirmedBlocks(k int) []*IndexedBlock {
	b.Lock()
	defer b.Unlock()

	l := len(b.blocks)
	if l <= k {
		return nil
	}

	res := make([]*IndexedBlock, l-k)
	copy(res, b.blocks)
	b.blocks = b.blocks[l-k:]

	return res
}

// FindBlock uses binary search to find the block with the given height in cache
func (b *BTCCache) FindBlock(blockHeight uint64) *IndexedBlock {
	b.RLock()
	defer b.RUnlock()

	firstHeight := b.blocks[0].Height
	lastHeight := b.blocks[len(b.blocks)-1].Height
	if int32(blockHeight) < firstHeight || lastHeight < int32(blockHeight) {
		return nil
	}

	leftBound := uint64(0)
	rightBound := b.size() - 1

	for leftBound <= rightBound {
		midPoint := leftBound + (rightBound-leftBound)/2

		if b.blocks[midPoint].Height == int32(blockHeight) {
			return b.blocks[midPoint]
		}

		if b.blocks[midPoint].Height > int32(blockHeight) {
			rightBound = midPoint - 1
		} else {
			leftBound = midPoint + 1
		}
	}

	return nil
}

func (b *BTCCache) Resize(maxEntries uint64) error {
	b.Lock()
	defer b.Unlock()

	if maxEntries == 0 {
		return ErrInvalidMaxEntries
	}
	b.maxEntries = maxEntries
	return nil
}

// Trim trims BTCCache to only keep the latest `maxEntries` blocks, and set `maxEntries` to be the cache size
func (b *BTCCache) Trim() {
	b.Lock()
	defer b.Unlock()

	// cache size is smaller than maxEntries, can't trim
	if b.size() < b.maxEntries {
		return
	}

	// dereference b.blocks[:len(b.blocks)-int(b.maxEntries)] to ensure they will be garbage-collected
	for i := range b.blocks[:len(b.blocks)-int(b.maxEntries)] {
		b.blocks[i] = nil
	}

	b.blocks = b.blocks[len(b.blocks)-int(b.maxEntries):]
}
