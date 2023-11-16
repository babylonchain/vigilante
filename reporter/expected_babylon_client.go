package reporter

import (
	btcctypes "github.com/babylonchain/babylon/x/btccheckpoint/types"
	btclctypes "github.com/babylonchain/babylon/x/btclightclient/types"
	"github.com/babylonchain/rpc-client/config"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type BabylonClient interface {
	MustGetAddr() sdk.AccAddress
	GetConfig() *config.BabylonConfig
	BTCCheckpointParams() (*btcctypes.QueryParamsResponse, error)
	InsertHeaders(msgs *btclctypes.MsgInsertHeaders) (*sdk.TxResponse, error)
	ContainsBTCBlock(blockHash *chainhash.Hash) (*btclctypes.QueryContainsBytesResponse, error)
	BTCHeaderChainTip() (*btclctypes.QueryTipResponse, error)
	BTCBaseHeader() (*btclctypes.QueryBaseHeaderResponse, error)
	InsertBTCSpvProof(msg *btcctypes.MsgInsertBTCSpvProof) (*sdk.TxResponse, error)
	Stop()
}
