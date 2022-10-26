package types

import (
	"fmt"

	"github.com/babylonchain/babylon/btctxformatter"
	btcctypes "github.com/babylonchain/babylon/x/btccheckpoint/types"
)

type Ckpt struct {
	Segments []*CkptSegment
	Epoch    uint64
}

func NewCkpt(ckptSeg1 *CkptSegment, ckptSeg2 *CkptSegment, epochNumber uint64) *Ckpt {
	return &Ckpt{
		Segments: []*CkptSegment{ckptSeg1, ckptSeg2},
		Epoch:    epochNumber,
	}
}

func (ckpt *Ckpt) GenSPVProofs() ([]*btcctypes.BTCSpvProof, error) {
	if len(ckpt.Segments) != btctxformatter.NumberOfParts {
		return nil, fmt.Errorf("unexpected number of txs in a pair: got %d, want %d", len(ckpt.Segments), btctxformatter.NumberOfParts)
	}
	proofs := []*btcctypes.BTCSpvProof{}
	for _, ckptSeg := range ckpt.Segments {
		proof, err := ckptSeg.AssocBlock.GenSPVProof(ckptSeg.TxIdx)
		if err != nil {
			return nil, err
		}
		proofs = append(proofs, proof)
	}
	return proofs, nil
}
