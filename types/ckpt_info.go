package types

import (
	"time"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/wire"
)

// CheckpointInfo stores information of a BTC checkpoint
type CheckpointInfo struct {
	Epoch uint64
	Ts    time.Time // the timestamp of the checkpoint being sent
	Tx1   *BtcTxInfo
	Tx2   *BtcTxInfo
}

// BtcTxInfo stores information of a BTC tx as part of a checkpoint
type BtcTxInfo struct {
	Tx            *wire.MsgTx
	ChangeAddress btcutil.Address
	Size          uint64 // the size of the BTC tx
	UtxoAmount    uint64 // the amount of the UTXO used in the BTC tx
}
