package babylonclient

import (
	"context"

	btcctypes "github.com/babylonchain/babylon/x/btccheckpoint/types"
	btcltypes "github.com/babylonchain/babylon/x/btclightclient/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (c *Client) InsertBTCSpvProof(msg *btcctypes.MsgInsertBTCSpvProof) (res *sdk.TxResponse, err error) {
	// generate context
	// TODO: what should be put in the context?
	ctx := context.TODO()
	return c.SendMsg(ctx, msg)
}

func (c *Client) InsertHeader(msg *btcltypes.MsgInsertHeader) (res *sdk.TxResponse, err error) {
	// generate context
	// TODO: what should be put in the context?
	ctx := context.TODO()
	return c.SendMsg(ctx, msg)
}

// TODO: implement necessary message invocations here
// - MsgInconsistencyEvidence
// - MsgStallingEvidence
