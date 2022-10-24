package types

import (
	"fmt"
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

func (b *BTCCache) Init(ibs []*IndexedBlock) error {
	b.Lock()
	defer b.Unlock()

	if len(ibs) > int(b.maxEntries) {
		return ErrTooManyEntries
	}
	for _, ib := range ibs {
		if err := b.add(ib); err != nil {
			return err
		}
	}

	return b.reverse()
}

// Add adds a new block to the cache. Thread-safe.
func (b *BTCCache) Add(ib *IndexedBlock) error {
	b.Lock()
	defer b.Unlock()

	if b.size() >= b.maxEntries {
		b.blocks = b.blocks[1:]
	}

	b.blocks = append(b.blocks, ib)
	return nil
}

// Lock free version of Add
func (b *BTCCache) add(ib *IndexedBlock) error {
	if b.size() >= b.maxEntries {
		b.blocks = b.blocks[1:]
	}

	b.blocks = append(b.blocks, ib)
	return nil
}

func (b *BTCCache) Tip() (*IndexedBlock, error) {
	b.RLock()
	defer b.RUnlock()

	if b.size() == 0 {
		return nil, ErrEmptyCache
	}

	return b.blocks[len(b.blocks)-1], nil
}

// RemoveLast deletes the last block in cache
func (b *BTCCache) RemoveLast() error {
	b.Lock()
	defer b.Unlock()

	if b.size() == 0 {
		return ErrEmptyCache
	}

	b.blocks = b.blocks[:len(b.blocks)-1]
	return nil
}

// Size returns the size of the cache. Thread-safe.
func (b *BTCCache) Size() uint64 {
	b.RLock()
	defer b.RUnlock()

	return uint64(len(b.blocks))
}

// lock free version of Size
func (b *BTCCache) size() uint64 {
	return uint64(len(b.blocks))
}

// Reverse reverses the order of blocks in cache in place
func (b *BTCCache) Reverse() error {
	b.Lock()
	defer b.Unlock()

	for i, j := 0, len(b.blocks)-1; i < j; i, j = i+1, j-1 {
		b.blocks[i], b.blocks[j] = b.blocks[j], b.blocks[i]
	}

	return nil
}

// lock free version of reverse
func (b *BTCCache) reverse() error {
	for i, j := 0, len(b.blocks)-1; i < j; i, j = i+1, j-1 {
		b.blocks[i], b.blocks[j] = b.blocks[j], b.blocks[i]
	}

	return nil
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

// FindBlock finds block at the given height in cache
func (b *BTCCache) FindBlock(blockHeight uint64) *IndexedBlock {
	b.RLock()
	defer b.RUnlock()

	firstHeight := b.blocks[0].Height
	lastHeight := b.blocks[len(b.blocks)-1].Height
	if int32(blockHeight) < firstHeight || lastHeight < int32(blockHeight) {
		return nil
	}

	for i := len(b.blocks) - 1; i >= 0; i-- {
		if b.blocks[i].Height == int32(blockHeight) {
			return b.blocks[i]
		}
	}

	return nil
}

// Trim trims BTCCache to only keep the latest `maxEntries` blocks, and set `maxEntries` to be the cache size
func (b *BTCCache) Trim(maxEntries uint64) error {
	b.Lock()
	defer b.Unlock()

	// if maxEntries is 0, it means that the cache is disabled
	if maxEntries == 0 {
		return ErrInvalidMaxEntries
	}

	// set maxEntries to be the cache size
	b.maxEntries = maxEntries

	b.blocks = b.blocks[len(b.blocks)-int(maxEntries):]
	return nil
}
