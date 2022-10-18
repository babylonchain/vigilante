package types

import (
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/wire"
)

type NotificationType int

const (
	// NTBlockDisconnected indicates the associated block was disconnected
	// from the main chain.
	NTBlockDisconnected NotificationType = iota

	// NTBlockConnected indicates the associated block was connected to the
	// main chain.
	NTBlockConnected
)

type NotificationMsg struct {
	Type   NotificationType
	Height int32
	Header *wire.BlockHeader
	Txs    []*btcutil.Tx
}

func NewNotificationMsg(ntype NotificationType, height int32, header *wire.BlockHeader, txs []*btcutil.Tx) *NotificationMsg {
	return &NotificationMsg{
		Type:   ntype,
		Height: height,
		Header: header,
		Txs:    txs,
	}
}
