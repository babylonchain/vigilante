package types

import (
	"fmt"

	"github.com/babylonchain/babylon/btctxformatter"
	btcctypes "github.com/babylonchain/babylon/x/btccheckpoint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// MustNewMsgInsertBTCSpvProof returns a MsgInsertBTCSpvProof msg given the submitter address and SPV proofs of two BTC txs
func MustNewMsgInsertBTCSpvProof(submitter sdk.AccAddress, proofs []*btcctypes.BTCSpvProof) *btcctypes.MsgInsertBTCSpvProof {
	var err error
	if len(proofs) != btctxformatter.NumberOfParts {
		err = fmt.Errorf("incorrect number of proofs: want %d, got %d", btctxformatter.NumberOfParts, len(proofs))
		panic(err)
	}

	return &btcctypes.MsgInsertBTCSpvProof{
		Submitter: submitter.String(),
		Proofs:    proofs,
	}
}
