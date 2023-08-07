package monitor

import (
	btclctypes "github.com/babylonchain/babylon/x/btclightclient/types"
	checkpointingtypes "github.com/babylonchain/babylon/x/checkpointing/types"
	epochingtypes "github.com/babylonchain/babylon/x/epoching/types"
	monitortypes "github.com/babylonchain/babylon/x/monitor/types"
	"github.com/babylonchain/vigilante/monitor/btcslasher"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	sdkquerytypes "github.com/cosmos/cosmos-sdk/types/query"
)

type BabylonQueryClient interface {
	btcslasher.BabylonQueryClient
	EndedEpochBTCHeight(epochNum uint64) (*monitortypes.QueryEndedEpochBtcHeightResponse, error)
	ReportedCheckpointBTCHeight(hashStr string) (*monitortypes.QueryReportedCheckpointBtcHeightResponse, error)
	RawCheckpoint(epochNumber uint64) (*checkpointingtypes.QueryRawCheckpointResponse, error)
	BTCHeaderChainTip() (*btclctypes.QueryTipResponse, error)
	ContainsBTCBlock(blockHash *chainhash.Hash) (*btclctypes.QueryContainsBytesResponse, error)
	CurrentEpoch() (*epochingtypes.QueryCurrentEpochResponse, error)
	BlsPublicKeyList(epochNumber uint64, pagination *sdkquerytypes.PageRequest) (*checkpointingtypes.QueryBlsPublicKeyListResponse, error) 
}
