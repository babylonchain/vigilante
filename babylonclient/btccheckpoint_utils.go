package babylonclient

import (
	"fmt"

	btcctypes "github.com/babylonchain/babylon/x/btccheckpoint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	NumExpectedProofs = 2
)

func NewMsgInsertBTCSpvProof(submitter sdk.AccAddress, proofs []*btcctypes.BTCSpvProof) (*btcctypes.MsgInsertBTCSpvProof, error) {
	// TODO: get proof stuff from BTC block
	if len(proofs) != NumExpectedProofs {
		return nil, fmt.Errorf("incorrect number of proofs: want %d, got %d", NumExpectedProofs, len(proofs))
	}

	return &btcctypes.MsgInsertBTCSpvProof{
		Submitter: submitter.String(),
		Proofs:    proofs,
	}, nil
}
