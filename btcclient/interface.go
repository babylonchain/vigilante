package btcclient

import (
	"github.com/babylonchain/vigilante/types"
	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

type BTCClient interface {
	Stop()
	WaitForShutdown()
	MustSubscribeBlocksByWebSocket()
	GetBestBlock() (*chainhash.Hash, uint64, error)
	GetBlockByHash(blockHash *chainhash.Hash) (*types.IndexedBlock, *wire.MsgBlock, error)
	GetLastBlocks(stopHeight uint64) ([]*types.IndexedBlock, error)
}

type BTCWallet interface {
	Stop()
	GetWalletName() string
	GetWalletPass() string
	GetWalletLockTime() int64
	GetNetParams() *chaincfg.Params
	GetTxFee() uint64 // in the unit of satoshi
	ListUnspent() ([]btcjson.ListUnspentResult, error)
	SendRawTransaction(tx *wire.MsgTx, allowHighFees bool) (*chainhash.Hash, error)
	GetRawChangeAddress(account string) (btcutil.Address, error)
	WalletPassphrase(passphrase string, timeoutSecs int64) error
	DumpPrivKey(address btcutil.Address) (*btcutil.WIF, error)
}
