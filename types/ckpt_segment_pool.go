package types

import (
	"crypto/sha256"
	"fmt"
	"sort"

	"github.com/babylonchain/babylon/btctxformatter"
)

type CkptSegmentPool struct {
	Tag     btctxformatter.BabylonTag
	Version btctxformatter.FormatVersion

	// first key: index of the segment in the checkpoint (0 or 1)
	// second key: hash of the OP_RETURN data in this ckpt segment
	Pool map[uint8]map[string]*CkptSegment
}

func NewCkptSegmentPool(tag btctxformatter.BabylonTag, version btctxformatter.FormatVersion) CkptSegmentPool {
	pool := map[uint8]map[string]*CkptSegment{}
	for i := uint8(0); i < btctxformatter.NumberOfParts; i++ {
		pool[i] = map[string]*CkptSegment{}
	}

	return CkptSegmentPool{
		Tag:     tag,
		Version: version,
		Pool:    pool,
	}
}

func (p *CkptSegmentPool) Add(ckptSeg *CkptSegment) error {
	if ckptSeg.Index >= btctxformatter.NumberOfParts {
		return fmt.Errorf("the index of the ckpt segment in block %v is out of scope: got %d, at most %d", ckptSeg.AssocBlock.BlockHash(), ckptSeg.Index, btctxformatter.NumberOfParts-1)
	}
	hash := sha256.Sum256(ckptSeg.Data)
	p.Pool[ckptSeg.Index][string(hash[:])] = ckptSeg
	return nil
}

// TODO: generalise to NumExpectedProofs > 2
// TODO: optimise the complexity by hashmap
func (p *CkptSegmentPool) Match() []*Ckpt {
	matchedCkpts := []*Ckpt{}

	for hash1, ckptSeg1 := range p.Pool[uint8(0)] {
		for hash2, ckptSeg2 := range p.Pool[uint8(1)] {
			if connected, err := btctxformatter.ConnectParts(p.Version, ckptSeg1.Data, ckptSeg2.Data); err == nil {
				// found a pair
				// Check that it is a valid checkpoint
				rawCheckpoint, err := btctxformatter.DecodeRawCheckpoint(p.Version, connected)
				if err != nil {
					continue
				}
				// create and append the checkpoint
				ckpt := NewCkpt(ckptSeg1, ckptSeg2, rawCheckpoint.Epoch)
				matchedCkpts = append(matchedCkpts, ckpt)
				// remove the two ckptSeg in pool
				delete(p.Pool[uint8(0)], hash1)
				delete(p.Pool[uint8(1)], hash2)
			}
		}
	}

	// Sort the matched pairs by epoch, since they have to be submitted in order
	sort.Slice(matchedCkpts, func(i, j int) bool {
		return matchedCkpts[i].Epoch < matchedCkpts[j].Epoch
	})
	return matchedCkpts
}
