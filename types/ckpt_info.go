package types

import (
	"time"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/wire"
)

type CheckpointInfo struct {
	Epoch uint64
	Ts    time.Time
	Tx1   *BtcTxInfo
	Tx2   *BtcTxInfo
}

// BtcTxInfo stores information of a BTC tx as part of a checkpoint
type BtcTxInfo struct {
	Tx            *wire.MsgTx
	ChangeAddress btcutil.Address
	Size          uint64
	UtxoAmount    uint64
}
