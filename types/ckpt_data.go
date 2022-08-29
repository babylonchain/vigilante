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

type CheckpointData struct {
	Index   uint8 // Index < NumExpectedProofs, e.g., 2 in BTC
	Data    []byte
	AssocTx *btcutil.Tx // the tx that contains the OP_RETURN data carrying a ckpt part
}

type CheckpointDataPool struct {
	Tag     btctxformatter.BabylonTag
	Version btctxformatter.FormatVersion
	Pool    map[string]*CheckpointData
}

// TODO: make the following params in config
func NewCheckpointDataPool(tag btctxformatter.BabylonTag, version btctxformatter.FormatVersion) CheckpointDataPool {
	return CheckpointDataPool{
		Tag:     tag,
		Version: version,
		Pool:    map[string]*CheckpointData{},
	}
}

func (p *CheckpointDataPool) Add(ckptData *CheckpointData) error {
	if ckptData.Index >= btctxformatter.NumberOfParts {
		return fmt.Errorf("the index of ckptData in tx %v is out of scope: got %d, at most %d", ckptData.AssocTx.Hash(), ckptData.Index, btctxformatter.NumberOfParts-1)
	}
	hash := sha256.Sum256(ckptData.Data)
	p.Pool[string(hash[:])] = ckptData
	return nil
}

// TODO: generalise to NumExpectedProofs > 2
// TODO: optimise the complexity by hashmap
func (p *CheckpointDataPool) Match() [][]*btcutil.Tx {
	matchedPairs := [][]*btcutil.Tx{}

	for hash1, ckptData1 := range p.Pool {
		for hash2, ckptData2 := range p.Pool {
			if _, err := btctxformatter.ConnectParts(p.Version, ckptData1.Data, ckptData2.Data); err == nil {
				// found a pair
				// append the tx pair
				pair := []*btcutil.Tx{ckptData1.AssocTx, ckptData2.AssocTx}
				matchedPairs = append(matchedPairs, pair)
				// remove the two ckptData in pool
				delete(p.Pool, hash1)
				delete(p.Pool, hash2)
			}
		}
	}
	return matchedPairs
}

func TxPairToSPVProofs(ib *IndexedBlock, pair []*btcutil.Tx) ([]*btcctypes.BTCSpvProof, error) {
	if len(pair) != btctxformatter.NumberOfParts {
		return nil, fmt.Errorf("Unexpected number of txs in a pair: got %d, want %d", len(pair), btctxformatter.NumberOfParts)
	}
	proofs := []*btcctypes.BTCSpvProof{}
	for _, tx := range pair {
		proof, err := ib.GenSPVProof(tx.Index())
		if err != nil {
			return nil, err
		}
		proofs = append(proofs, proof)
	}
	return proofs, nil
}

func GetCkptData(tag btctxformatter.BabylonTag, version btctxformatter.FormatVersion, tx *btcutil.Tx) *CheckpointData {
	opReturnData := extractOpReturnData(tx.MsgTx())

	idx := uint8(0)
	for idx < btctxformatter.NumberOfParts {
		data, err := btctxformatter.GetCheckpointData(tag, version, idx, opReturnData)
		if err == nil {
			ckptData := &CheckpointData{
				Index:   idx,
				Data:    data,
				AssocTx: tx,
			}
			return ckptData
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
