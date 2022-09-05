package types

type BTCCache struct {
	blocks     []*IndexedBlock
	MaxEntries uint
}

func NewBTCCache(maxEntries uint) *BTCCache {
	return &BTCCache{
		blocks:     make([]*IndexedBlock, 0, maxEntries),
		MaxEntries: maxEntries,
	}
}

func (b *BTCCache) Add(ib *IndexedBlock) {
	if b.MaxEntries == 0 {
		return
	}

	if uint(len(b.blocks)) == b.MaxEntries {
		b.blocks = b.blocks[1:]
	}

	b.blocks = append(b.blocks, ib)
}

func (b *BTCCache) Size() int {
	return len(b.blocks)
}

func (b *BTCCache) Reverse() {
	for i, j := 0, len(b.blocks)-1; i < j; i, j = i+1, j-1 {
		b.blocks[i], b.blocks[j] = b.blocks[j], b.blocks[i]
	}
}
