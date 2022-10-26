package babylonclient

import (
	"context"
	"fmt"

	"github.com/babylonchain/babylon/types/retry"
	btcctypes "github.com/babylonchain/babylon/x/btccheckpoint/types"
	btclctypes "github.com/babylonchain/babylon/x/btclightclient/types"
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

func (c *Client) MustInsertBTCSpvProof(msg *btcctypes.MsgInsertBTCSpvProof) *sdk.TxResponse {
	var (
		res *sdk.TxResponse
		err error
	)
	err = retry.Do(c.retrySleepTime, c.maxRetrySleepTime, func() error {
		res, err = c.InsertBTCSpvProof(msg)
		return err
	})
	if err != nil {
		panic(fmt.Errorf("failed to insert new MsgInsertBTCSpvProof: %v", err))
	}
	return res
}

func (c *Client) InsertHeader(msg *btclctypes.MsgInsertHeader) (*sdk.TxResponse, error) {
	// generate context
	// TODO: what should be put in the context?
	// ctx, cancelCtx := context.WithTimeout(context.TODO(), 30*time.Second)
	// defer cancelCtx()
	ctx := context.TODO()
	res, err := c.SendMsg(ctx, msg)
	ctx.Done()

	return res, err
}

func (c *Client) InsertHeaders(msgs []*btclctypes.MsgInsertHeader) (*sdk.TxResponse, error) {
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
