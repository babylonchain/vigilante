package types

import (
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcd/btcutil"
)

type (
	SupportedBtcNetwork string
	SupportedBtcBackend string
)

const (
	BtcMainnet SupportedBtcNetwork = "mainnet"
	BtcTestnet SupportedBtcNetwork = "testnet"
	BtcSimnet  SupportedBtcNetwork = "simnet"
	BtcRegtest SupportedBtcNetwork = "regtest"
	BtcSignet  SupportedBtcNetwork = "signet"

	Btcd     SupportedBtcBackend = "btcd"
	Bitcoind SupportedBtcBackend = "bitcoind"
)

func (c SupportedBtcNetwork) String() string {
	return string(c)
}

func (c SupportedBtcBackend) String() string {
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

func GetValidBtcBackends() map[SupportedBtcBackend]bool {
	validBtcBackends := map[SupportedBtcBackend]bool{
		Bitcoind: true,
		Btcd:     true,
	}

	return validBtcBackends
}
