package types

import (
	"crypto/sha256"
	"fmt"
	"sort"

	"github.com/babylonchain/babylon/btctxformatter"
)

type CheckpointCache struct {
	Tag     btctxformatter.BabylonTag
	Version btctxformatter.FormatVersion

	// list that contains matched checkpoints
	Checkpoints []*Ckpt

	// map that contains checkpoint segments
	// first key: index of the segment in the checkpoint (0 or 1)
	// second key: hash of the OP_RETURN data in this ckpt segment
	Segments map[uint8]map[string]*CkptSegment
}

func NewCheckpointCache(tag btctxformatter.BabylonTag, version btctxformatter.FormatVersion) CheckpointCache {
	segMap := map[uint8]map[string]*CkptSegment{}
	for i := uint8(0); i < btctxformatter.NumberOfParts; i++ {
		segMap[i] = map[string]*CkptSegment{}
	}
	ckptList := []*Ckpt{}

	return CheckpointCache{
		Tag:         tag,
		Version:     version,
		Checkpoints: ckptList,
		Segments:    segMap,
	}
}

func (c *CheckpointCache) AddSegment(ckptSeg *CkptSegment) error {
	if ckptSeg.Index >= btctxformatter.NumberOfParts {
		return fmt.Errorf("the index of the ckpt segment in block %v is out of scope: got %d, at most %d", ckptSeg.AssocBlock.BlockHash(), ckptSeg.Index, btctxformatter.NumberOfParts-1)
	}
	hash := sha256.Sum256(ckptSeg.Data)
	c.Segments[ckptSeg.Index][string(hash[:])] = ckptSeg
	return nil
}

func (c *CheckpointCache) AddCheckpoint(ckpt *Ckpt) {
	c.Checkpoints = append(c.Checkpoints, ckpt)
}

func (c *CheckpointCache) sortCheckpoints() {
	// Sort the matched pairs by epoch, since they have to be submitted in order
	// TODO: find smarter way for sorting
	sort.Slice(c.Checkpoints, func(i, j int) bool {
		return c.Checkpoints[i].Epoch < c.Checkpoints[j].Epoch
	})
}

// TODO: generalise to NumExpectedProofs > 2
// TODO: optimise the complexity by hashmap
func (c *CheckpointCache) Match() {
	for hash1, ckptSeg1 := range c.Segments[uint8(0)] {
		for hash2, ckptSeg2 := range c.Segments[uint8(1)] {
			connected, err := btctxformatter.ConnectParts(c.Version, ckptSeg1.Data, ckptSeg2.Data)
			if err != nil {
				continue
			}
			// found a pair, check if it is a valid checkpoint
			rawCheckpoint, err := btctxformatter.DecodeRawCheckpoint(c.Version, connected)
			if err != nil {
				continue
			}
			// create the matched checkpoint
			ckpt := NewCkpt(ckptSeg1, ckptSeg2, rawCheckpoint.Epoch)
			// add to the ckptList
			c.AddCheckpoint(ckpt)
			// remove the two ckptSeg in segMap
			delete(c.Segments[uint8(0)], hash1)
			delete(c.Segments[uint8(1)], hash2)
		}
	}

	// this ensures that checkpoints in the cache is always in order
	c.sortCheckpoints()
}

func (c *CheckpointCache) PopEarliestCheckpoint() *Ckpt {
	if c.HasCheckpoints() {
		ckpt := c.Checkpoints[0]
		c.Checkpoints = c.Checkpoints[1:]
		return ckpt
	} else {
		return nil
	}
}

func (c *CheckpointCache) NumSegments() int {
	size := 0
	for _, segMap := range c.Segments {
		size += len(segMap)
	}
	return size
}

func (c *CheckpointCache) NumCheckpoints() int {
	return len(c.Checkpoints)
}

func (c *CheckpointCache) HasCheckpoints() bool {
	return c.NumCheckpoints() > 0
}
