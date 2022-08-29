package types

import (
	"fmt"

	btcctypes "github.com/babylonchain/babylon/x/btccheckpoint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// adapted from https://github.com/babylonchain/babylon/blob/648b804bc492ded2cb826ba261d7164b4614d78a/x/btccheckpoint/btcutils/btcutils.go
	NumExpectedProofs       = 2
	MaxOpReturnPkScriptSize = 83
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
