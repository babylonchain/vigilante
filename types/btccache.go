package types

import "fmt"

type BTCCache struct {
	blocks     []*IndexedBlock
	maxEntries uint
}

func NewBTCCache(maxEntries uint) *BTCCache {
	return &BTCCache{
		blocks:     make([]*IndexedBlock, 0, maxEntries),
		maxEntries: maxEntries,
	}
}

func (b *BTCCache) Init(ibs []*IndexedBlock) error {
	if len(ibs) > int(b.maxEntries) {
		return fmt.Errorf("the number of blocks is more than maxEntries")
	}
	for _, ib := range ibs {
		b.Add(ib)
	}
	b.reverse()
	return nil
}

func (b *BTCCache) Add(ib *IndexedBlock) {
	if b.maxEntries == 0 {
		return
	}

	if uint(len(b.blocks)) == b.maxEntries {
		b.blocks = b.blocks[1:]
	}

	b.blocks = append(b.blocks, ib)
}

func (b *BTCCache) Size() uint64 {
	return uint64(len(b.blocks))
}

func (b *BTCCache) reverse() {
	for i, j := 0, len(b.blocks)-1; i < j; i, j = i+1, j-1 {
		b.blocks[i], b.blocks[j] = b.blocks[j], b.blocks[i]
	}
}

// GetLastBlocks returns list of blocks from cache up to a specified height
func (b *BTCCache) GetLastBlocks(stopHeight uint64) []*IndexedBlock {
	if b.blocks[0].Height <= int32(stopHeight) {
		return b.blocks
	}
	if b.blocks[len(b.blocks)-1].Height > int32(stopHeight) {
		return []*IndexedBlock{}
	}

	var j int
	for i := len(b.blocks) - 1; i >= 0; i-- {
		if b.blocks[i].Height < int32(stopHeight) {
			j = i
			break
		}
	}

	return b.blocks[j:]
}

// FindBlock finds block in cache with given height
func (b *BTCCache) FindBlock(blockHeight uint64) *IndexedBlock {
	if b.blocks[0].Height < int32(blockHeight) || b.blocks[len(b.blocks)-1].Height > int32(blockHeight) {
		return nil
	}
	for i := len(b.blocks) - 1; i >= 0; i-- {
		if b.blocks[i].Height == int32(blockHeight) {
			return b.blocks[i]
		}
	}

	return nil
}
