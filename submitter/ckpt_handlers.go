package submitter

import (
	"github.com/babylonchain/babylon/x/checkpointing/types"
	"github.com/btcsuite/btcd/wire"
)

func (r *Submitter) sealedCkptHandler() {
	// TODO: event-loop for sealed checkpoints
}

func (r *Submitter) submitCkpt(ckpt types.RawCheckpoint) error {
	// TODO: 1. convert ckpt into two raw txs; 2. send txs to BTC
	return nil
}

func (r *Submitter) convertCkptToRawTx(ckpt types.RawCheckpoint) (*wire.MsgTx, *wire.MsgTx, error) {
	panic("Implement me")
}

func (r *Submitter) sendTxToBTC(tx *wire.MsgTx) error {
	panic("Implement me")
}
