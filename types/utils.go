package types

import (
	"github.com/btcsuite/btcutil"
	"github.com/btcsuite/btcd/wire"
)

type (
	SupportedBtcNetwork       string
	SupportedSubscriptionMode string
)

const (
	BtcMainnet SupportedBtcNetwork = "mainnet"
	BtcTestnet SupportedBtcNetwork = "testnet"
	BtcSimnet  SupportedBtcNetwork = "simnet"
	BtcRegtest SupportedBtcNetwork = "regtest"
	BtcSignet  SupportedBtcNetwork = "signet"
)


	WebsocketMode SupportedSubscriptionMode = "websocket"
	ZmqMode       SupportedSubscriptionMode = "zmq"
)

func (c SupportedBtcNetwork) String() string {
	return string(c)
}

func (c SupportedSubscriptionMode) String() string {
	return string(c)
}

func GetWrappedTxs(msg *wire.MsgBlock) []*btcutil.Tx {
	btcTxs := []*btcutil.Tx{}

	for i := range msg.Transactions {
		newTx := btcutil.NewTx(msg.Transactions[i])
		newTx.SetIndex(i)

		btcTxs = append(btcTxs, newTx)
	}

	return btcTxs
}

func GetValidNetParams() map[string]bool {
	params := map[string]bool{
		BtcMainnet.String(): true,
		BtcTestnet.String(): true,
		BtcSimnet.String():  true,
		BtcRegtest.String(): true,
		BtcSignet.String():  true,
	}

	return params
}

func GetValidSubscriptionModes() map[SupportedSubscriptionMode]bool {
	validSubscriptionModes := map[SupportedSubscriptionMode]bool{
		ZmqMode:       true,
		WebsocketMode: true,
	}

	return validSubscriptionModes
}
