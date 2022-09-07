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

func (b *BTCCache) Size() int {
	return len(b.blocks)
}

func (b *BTCCache) GetBlocks(stopHeight uint64) []*IndexedBlock {

	var j int
	for i := len(b.blocks) - 1; i >= 0; i-- {
		if b.blocks[i].Height < int32(stopHeight) {
			j = i
		}
	}

	return b.blocks[j+1:]
}

func (b *BTCCache) reverse() {
	for i, j := 0, len(b.blocks)-1; i < j; i, j = i+1, j-1 {
		b.blocks[i], b.blocks[j] = b.blocks[j], b.blocks[i]
	}
}
