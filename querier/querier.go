package querier

import (
	"fmt"

	types2 "github.com/babylonchain/babylon/x/btccheckpoint/types"
	ckpttypes "github.com/babylonchain/babylon/x/checkpointing/types"
	bbnrpccli "github.com/babylonchain/rpc-client/query"
	"github.com/btcsuite/btcd/chaincfg/chainhash"

	"github.com/babylonchain/vigilante/log"
	"github.com/babylonchain/vigilante/types"
)

type BabylonQuerier struct {
	babylonCli bbnrpccli.BabylonQueryClient
}

func New(babylonCli bbnrpccli.BabylonQueryClient) *BabylonQuerier {
	return &BabylonQuerier{babylonCli: babylonCli}
}

// QueryInfoForNextEpoch fetches necessary information for verifying the next epoch from Babylon
func (q *BabylonQuerier) QueryInfoForNextEpoch(epoch uint64) (*types.EpochInfo, error) {
	// query validator set with BLS
	res, err := q.babylonCli.BlsPublicKeyList(epoch, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to query BLS key set for epoch %v: %w", epoch, err)
	}
	valSet := make([]*ckpttypes.ValidatorWithBlsKey, len(res.ValidatorWithBlsKeys))
	for i, key := range res.ValidatorWithBlsKeys {
		val := &ckpttypes.ValidatorWithBlsKey{
			ValidatorAddress: key.ValidatorAddress,
			BlsPubKey:        key.BlsPubKey,
			VotingPower:      key.VotingPower,
		}
		valSet[i] = val
	}

	ei := types.NewEpochInfo(epoch, ckpttypes.ValidatorWithBlsKeySet{ValSet: valSet})

	return ei, nil
}

// FindTipConfirmedEpoch tries to find the last confirmed epoch number from Babylon
func (q *BabylonQuerier) FindTipConfirmedEpoch() (uint64, error) {
	epochRes, err := q.babylonCli.CurrentEpoch()
	if err != nil {
		return 0, fmt.Errorf("failed to query the current epoch of Babylon: %w", err)
	}
	curEpoch := epochRes.CurrentEpoch
	log.Logger.Debugf("current epoch number is %v", curEpoch)
	for curEpoch >= 1 {
		ckptRes, err := q.babylonCli.RawCheckpoint(curEpoch - 1)
		if err != nil {
			return 0, fmt.Errorf("failed to query the checkpoint of epoch %v: %w", curEpoch-1, err)
		}
		if ckptRes.RawCheckpoint.Status == ckpttypes.Confirmed || ckptRes.RawCheckpoint.Status == ckpttypes.Finalized {
			return curEpoch - 1, nil
		}
		curEpoch--
	}

	return 0, fmt.Errorf("cannot find a confirmed or finalized epoch from Babylon")
}

func (q *BabylonQuerier) RawCheckpoint(epoch uint64) (*ckpttypes.RawCheckpointWithMeta, error) {
	res, err := q.babylonCli.RawCheckpoint(epoch)
	return res.RawCheckpoint, err
}

func (q *BabylonQuerier) RawCheckpointList(status ckpttypes.CheckpointStatus) ([]*ckpttypes.RawCheckpointWithMeta, error) {
	res, err := q.babylonCli.RawCheckpointList(status, nil)
	return res.RawCheckpoints, err
}

func (q *BabylonQuerier) ContainsBTCHeader(hash *chainhash.Hash) (bool, error) {
	res, err := q.babylonCli.ContainsBTCBlock(hash)
	return res.Contains, err
}

func (q *BabylonQuerier) BTCHeaderChainTip() (*chainhash.Hash, uint64, error) {
	res, err := q.babylonCli.BTCHeaderChainTip()
	return res.Header.Hash.ToChainhash(), res.Header.Height, err
}

func (q *BabylonQuerier) BTCBaseHeader() (*chainhash.Hash, uint64, error) {
	res, err := q.babylonCli.BTCBaseHeader()
	return res.Header.Hash.ToChainhash(), res.Header.Height, err
}

func (q *BabylonQuerier) EndedEpochBtcHeight(epochNum uint64) (uint64, error) {
	res, err := q.babylonCli.EndedEpochBTCHeight(epochNum)
	return res.BtcLightClientHeight, err
}

func (q *BabylonQuerier) ReportedCheckpointBtcHeight(id string) (uint64, error) {
	res, err := q.babylonCli.ReportedCheckpointBTCHeight(id)
	return res.BtcLightClientHeight, err
}

func (q *BabylonQuerier) BTCCheckpointParams() (types2.Params, error) {
	res, err := q.babylonCli.BTCCheckpointParams()
	return res.Params, err
}
