package types

import (
	"fmt"

	btcctypes "github.com/babylonchain/babylon/x/btccheckpoint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	NumExpectedProofs = 2
)

// NewMsgInsertBTCSpvProof returns a MsgInsertBTCSpvProof msg given the submitter address and SPV proofs of two BTC txs
func NewMsgInsertBTCSpvProof(submitter sdk.AccAddress, proofs []*btcctypes.BTCSpvProof) (*btcctypes.MsgInsertBTCSpvProof, error) {
	if len(proofs) != NumExpectedProofs {
		return nil, fmt.Errorf("incorrect number of proofs: want %d, got %d", NumExpectedProofs, len(proofs))
	}

	return &btcctypes.MsgInsertBTCSpvProof{
		Submitter: submitter.String(),
		Proofs:    proofs,
	}, nil
}
