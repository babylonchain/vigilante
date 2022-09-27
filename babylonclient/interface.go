package babylonclient

import (
	btcctypes "github.com/babylonchain/babylon/x/btccheckpoint/types"
	btclctypes "github.com/babylonchain/babylon/x/btclightclient/types"
	checkpointingtypes "github.com/babylonchain/babylon/x/checkpointing/types"
	epochingtypes "github.com/babylonchain/babylon/x/epoching/types"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

type BabylonClient interface {
	Stop()
	GetTagIdx() uint8
	GetAddr() (sdk.AccAddress, error)
	MustGetAddr() sdk.AccAddress
	QueryStakingParams() (*stakingtypes.Params, error)
	QueryEpochingParams() (*epochingtypes.Params, error)
	QueryBTCLightclientParams() (*btclctypes.Params, error)
	QueryBTCCheckpointParams() (*btcctypes.Params, error)
	MustQueryBTCCheckpointParams() *btcctypes.Params
	QueryHeaderChainTip() (*chainhash.Hash, uint64, error)
	QueryRawCheckpoint(epochNumber uint64) (*checkpointingtypes.RawCheckpointWithMeta, error)
	QueryRawCheckpointList(status checkpointingtypes.CheckpointStatus) ([]*checkpointingtypes.RawCheckpointWithMeta, error)
	QueryBaseHeader() (*wire.BlockHeader, uint64, error)
	QueryContainsBlock(blockHash *chainhash.Hash) (bool, error)
	InsertBTCSpvProof(msg *btcctypes.MsgInsertBTCSpvProof) (*sdk.TxResponse, error)
	InsertHeader(msg *btclctypes.MsgInsertHeader) (*sdk.TxResponse, error)
}
