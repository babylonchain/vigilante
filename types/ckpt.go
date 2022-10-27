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

func (ckpt *Ckpt) MustGenSPVProofs() []*btcctypes.BTCSpvProof {
	var (
		err    error
		proofs []*btcctypes.BTCSpvProof
	)
	if len(ckpt.Segments) != btctxformatter.NumberOfParts {
		err = fmt.Errorf("incorrect number of segments: want %d, got %d", btctxformatter.NumberOfParts, len(ckpt.Segments))
		panic(err)
	}

	for _, ckptSeg := range ckpt.Segments {
		proof, err := ckptSeg.AssocBlock.GenSPVProof(ckptSeg.TxIdx)
		if err != nil {
			panic(err)
		}
		proofs = append(proofs, proof)
	}

	return proofs
}
