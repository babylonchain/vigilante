package types

import (
	"crypto/sha256"
	"fmt"

	"github.com/babylonchain/babylon/btctxformatter"
	btcctypes "github.com/babylonchain/babylon/x/btccheckpoint/types"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

type CkptSegment struct {
	Index      uint8  // Index < NumExpectedProofs, e.g., 2 in BTC
	Data       []byte // OP_RETURN data *excluding* the Babylon header
	TxIdx      int    // index of the tx in AssocBlock that contains the OP_RETURN data carrying a ckpt segment
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
	opReturnData := extractOpReturnData(tx.MsgTx())

	idx := uint8(0)
	for idx < btctxformatter.NumberOfParts {
		data, err := btctxformatter.GetCheckpointData(tag, version, idx, opReturnData)
		if err == nil {
			ckptSeg := &CkptSegment{
				Index:      idx,
				Data:       data,
				TxIdx:      tx.Index(),
				AssocBlock: block,
			}
			return ckptSeg
		}
		idx++
	}
	return nil
}

// adapted from https://github.com/babylonchain/babylon/blob/648b804bc492ded2cb826ba261d7164b4614d78a/x/btccheckpoint/btcutils/btcutils.go#L105-L131
func extractOpReturnData(msgTx *wire.MsgTx) []byte {
	opReturnData := []byte{}

	for _, output := range msgTx.TxOut {
		pkScript := output.PkScript
		pkScriptLen := len(pkScript)
		// valid op return script will have at least 2 bytes
		// - fisrt byte should be OP_RETURN marker
		// - second byte should indicate how many bytes there are in opreturn script
		if pkScriptLen > 1 &&
			pkScriptLen <= MaxOpReturnPkScriptSize &&
			pkScript[0] == txscript.OP_RETURN {

			// if this is OP_PUSHDATA1, we need to drop first 3 bytes as those are related
			// to script itself i.e OP_RETURN + OP_PUSHDATA1 + len of bytes
			if pkScript[1] == txscript.OP_PUSHDATA1 {
				opReturnData = append(opReturnData, pkScript[3:]...)
			} else {
				// this should be one of OP_DATAXX opcodes we drop first 2 bytes
				opReturnData = append(opReturnData, pkScript[2:]...)
			}
		}
	}

	return opReturnData
}
