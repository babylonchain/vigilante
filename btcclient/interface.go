package btcclient

import (
	"github.com/babylonchain/vigilante/types"
	"github.com/btcsuite/btcd/btcjson"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
)

type BTCClient interface {
	Stop()
	WaitForShutdown()
	MustSubscribeBlocks()
	BlockEventChan() <-chan *types.BlockEvent
	GetBestBlock() (*chainhash.Hash, uint64, error)
	GetBlockByHash(blockHash *chainhash.Hash) (*types.IndexedBlock, *wire.MsgBlock, error)
	FindTailBlocksUntilHeight(stopHeight uint64) ([]*types.IndexedBlock, error)
	GetChainBlocks(baseHeight uint64, tipBlock *types.IndexedBlock) ([]*types.IndexedBlock, error)
	FindTailBlocks(deep uint64) ([]*types.IndexedBlock, error)
}

type BTCWallet interface {
	Stop()
	GetWalletName() string
	GetWalletPass() string
	GetWalletLockTime() int64
	GetNetParams() *chaincfg.Params
	GetTxFee(txSize uint64) uint64 // in the unit of satoshi
	GetMaxTxFee() uint64           // in the unit of satoshi
	GetMinTxFee() uint64           // in the unit of satoshi
	ListUnspent() ([]btcjson.ListUnspentResult, error)
	ListReceivedByAddress() ([]btcjson.ListReceivedByAddressResult, error)
	SendRawTransaction(tx *wire.MsgTx, allowHighFees bool) (*chainhash.Hash, error)
	GetRawChangeAddress(account string) (btcutil.Address, error)
	WalletPassphrase(passphrase string, timeoutSecs int64) error
	DumpPrivKey(address btcutil.Address) (*btcutil.WIF, error)
}
