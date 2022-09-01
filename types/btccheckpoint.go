package types

import (
	"fmt"

	"github.com/babylonchain/babylon/btctxformatter"
	btcctypes "github.com/babylonchain/babylon/x/btccheckpoint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// NewMsgInsertBTCSpvProof returns a MsgInsertBTCSpvProof msg given the submitter address and SPV proofs of two BTC txs
func NewMsgInsertBTCSpvProof(submitter sdk.AccAddress, proofs []*btcctypes.BTCSpvProof) (*btcctypes.MsgInsertBTCSpvProof, error) {
	if len(proofs) != btctxformatter.NumberOfParts {
		return nil, fmt.Errorf("incorrect number of proofs: want %d, got %d", btctxformatter.NumberOfParts, len(proofs))
	}

	return &btcctypes.MsgInsertBTCSpvProof{
		Submitter: submitter.String(),
		Proofs:    proofs,
	}, nil
}
