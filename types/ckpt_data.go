package types

import (
	"crypto/sha256"
	"fmt"

	"github.com/babylonchain/babylon/btctxformatter"
	btcctypes "github.com/babylonchain/babylon/x/btccheckpoint/types"
	"github.com/btcsuite/btcd/btcutil"
)

// CkptSegment is a segment of the Babylon checkpoint, including
// - Data: actual OP_RETURN data excluding the Babylon header
// - Index: index of the segment in the checkpoint
// - TxIdx: index of the tx in AssocBlock
// - AssocBlock: pointer to the block that contains the tx that carries the ckpt segment
type CkptSegment struct {
	*btctxformatter.BabylonData
	TxIdx      int
	AssocBlock *IndexedBlock
}

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
func (p *CkptSegmentPool) Match() [][]*CkptSegment {
	matchedPairs := [][]*CkptSegment{}

	for hash1, ckptSeg1 := range p.Pool[uint8(0)] {
		for hash2, ckptSeg2 := range p.Pool[uint8(1)] {
			if _, err := btctxformatter.ConnectParts(p.Version, ckptSeg1.Data, ckptSeg2.Data); err == nil {
				// found a pair
				// append the tx pair
				pair := []*CkptSegment{ckptSeg1, ckptSeg2}
				matchedPairs = append(matchedPairs, pair)
				// remove the two ckptSeg in pool
				delete(p.Pool[uint8(0)], hash1)
				delete(p.Pool[uint8(1)], hash2)
			}
		}
	}
	return matchedPairs
}

func CkptSegPairToSPVProofs(pair []*CkptSegment) ([]*btcctypes.BTCSpvProof, error) {
	if len(pair) != btctxformatter.NumberOfParts {
		return nil, fmt.Errorf("Unexpected number of txs in a pair: got %d, want %d", len(pair), btctxformatter.NumberOfParts)
	}
	proofs := []*btcctypes.BTCSpvProof{}
	for _, ckptSeg := range pair {
		proof, err := ckptSeg.AssocBlock.GenSPVProof(ckptSeg.TxIdx)
		if err != nil {
			return nil, err
		}
		proofs = append(proofs, proof)
	}
	return proofs, nil
}

func GetIndexedCkptSeg(tag btctxformatter.BabylonTag, version btctxformatter.FormatVersion, block *IndexedBlock, tx *btcutil.Tx) *CkptSegment {
	bbnData := getBabylonDataFromTx(tag, version, tx)
	if bbnData != nil {
		return &CkptSegment{
			BabylonData: bbnData,
			TxIdx:       tx.Index(),
			AssocBlock:  block,
		}
	} else {
		return nil
	}
}

func getBabylonDataFromTx(tag btctxformatter.BabylonTag, version btctxformatter.FormatVersion, tx *btcutil.Tx) *btctxformatter.BabylonData {
	opReturnData := btcctypes.ExtractOpReturnData(tx)
	bbnData, err := btctxformatter.IsBabylonCheckpointData(tag, version, opReturnData)
	if err != nil {
		return nil
	} else {
		return bbnData
	}
}
