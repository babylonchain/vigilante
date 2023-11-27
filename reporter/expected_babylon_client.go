package reporter

import (
	"context"
	btcctypes "github.com/babylonchain/babylon/x/btccheckpoint/types"
	btclctypes "github.com/babylonchain/babylon/x/btclightclient/types"
	"github.com/babylonchain/rpc-client/config"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	pv "github.com/cosmos/relayer/v2/relayer/provider"
)

type BabylonClient interface {
	MustGetAddr() string
	GetConfig() *config.BabylonConfig
	BTCCheckpointParams() (*btcctypes.QueryParamsResponse, error)
	InsertHeaders(ctx context.Context, msgs *btclctypes.MsgInsertHeaders) (*pv.RelayerTxResponse, error)
	ContainsBTCBlock(blockHash *chainhash.Hash) (*btclctypes.QueryContainsBytesResponse, error)
	BTCHeaderChainTip() (*btclctypes.QueryTipResponse, error)
	BTCBaseHeader() (*btclctypes.QueryBaseHeaderResponse, error)
	InsertBTCSpvProof(ctx context.Context, msg *btcctypes.MsgInsertBTCSpvProof) (*pv.RelayerTxResponse, error)
	Stop() error
}
