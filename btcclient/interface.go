package btcclient

import (
	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"

	"github.com/babylonchain/vigilante/types"
)

type BTCClient interface {
	Stop()
	WaitForShutdown()
	MustSubscribeBlocks()
	BlockEventChan() <-chan *types.BlockEvent
	GetBestBlock() (*chainhash.Hash, uint64, error)
	GetBlockByHash(blockHash *chainhash.Hash) (*types.IndexedBlock, *wire.MsgBlock, error)
	FindTailBlocksByHeight(height uint64) ([]*types.IndexedBlock, error)
	GetBlockByHeight(height uint64) (*types.IndexedBlock, *wire.MsgBlock, error)
}

type BTCWallet interface {
	Stop()
	GetWalletName() string
	GetWalletPass() string
	GetWalletLockTime() int64
	GetNetParams() *chaincfg.Params
	GetTxFee(txSize uint64) uint64         // in the unit of satoshi
	GetMinTxRelayFee(txSize uint64) uint64 // in the unit of satoshi
	GetMaxTxFee(txSize uint64) uint64      // in the unit of satoshi
	GetMinTxFee(txSize uint64) uint64      // in the unit of satoshi
	ListUnspent() ([]btcjson.ListUnspentResult, error)
	ListReceivedByAddress() ([]btcjson.ListReceivedByAddressResult, error)
	SendRawTransaction(tx *wire.MsgTx, allowHighFees bool) (*chainhash.Hash, error)
	GetRawChangeAddress(account string) (btcutil.Address, error)
	WalletPassphrase(passphrase string, timeoutSecs int64) error
	DumpPrivKey(address btcutil.Address) (*btcutil.WIF, error)
}
