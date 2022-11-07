package types

import (
	"github.com/babylonchain/babylon/btctxformatter"
	btcctypes "github.com/babylonchain/babylon/x/btccheckpoint/types"
	"github.com/btcsuite/btcutil"
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

func NewCkptSegment(tag btctxformatter.BabylonTag, version btctxformatter.FormatVersion, block *IndexedBlock, tx *btcutil.Tx) *CkptSegment {
	opReturnData := btcctypes.ExtractOpReturnData(tx)
	bbnData, err := btctxformatter.IsBabylonCheckpointData(tag, version, opReturnData)
	if err != nil {
		return nil
	}
	return &CkptSegment{
		BabylonData: bbnData,
		TxIdx:       tx.Index(),
		AssocBlock:  block,
	}
}
