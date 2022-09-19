package babylonclient

import (
	"context"

	btcctypes "github.com/babylonchain/babylon/x/btccheckpoint/types"
	btcltypes "github.com/babylonchain/babylon/x/btclightclient/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (c *Client) InsertBTCSpvProof(msg *btcctypes.MsgInsertBTCSpvProof) (*sdk.TxResponse, error) {
	// generate context
	// TODO: what should be put in the context?
	// ctx, cancelCtx := context.WithTimeout(context.TODO(), 30*time.Second)
	// defer cancelCtx()
	ctx := context.TODO()
	res, err := c.SendMsg(ctx, msg)
	ctx.Done()

	return res, err
}

func (c *Client) InsertHeader(msg *btcltypes.MsgInsertHeader) (*sdk.TxResponse, error) {
	// generate context
	// TODO: what should be put in the context?
	// ctx, cancelCtx := context.WithTimeout(context.TODO(), 30*time.Second)
	// defer cancelCtx()
	ctx := context.TODO()
	res, err := c.SendMsg(ctx, msg)
	ctx.Done()

	return res, err
}

func (c *Client) InsertHeaders(msgs []*btcltypes.MsgInsertHeader) (*sdk.TxResponse, error) {
	// generate context
	// TODO: what should be put in the context?
	// ctx, cancelCtx := context.WithTimeout(context.TODO(), 30*time.Second)
	// defer cancelCtx()
	ctx := context.TODO()

	// convert to []sdk.Msg type
	imsgs := []sdk.Msg{}
	for _, msg := range msgs {
		imsgs = append(imsgs, msg)
	}

	res, err := c.SendMsgs(ctx, imsgs)
	ctx.Done()

	return res, err
}

// TODO: implement necessary message invocations here
// - MsgInconsistencyEvidence
// - MsgStallingEvidence
