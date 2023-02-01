package querier

import (
	"fmt"
	ckpttypes "github.com/babylonchain/babylon/x/checkpointing/types"
	bbnrpccli "github.com/babylonchain/rpc-client/client"
	"github.com/babylonchain/vigilante/log"
	"github.com/babylonchain/vigilante/types"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
)

type Querier struct {
	babylonCli bbnrpccli.BabylonClient
}

func New(babylonCli bbnrpccli.BabylonClient) *Querier {
	return &Querier{babylonCli: babylonCli}
}

// QueryInfoForNextEpoch fetches necessary information for verifying the next epoch from Babylon
func (q *Querier) QueryInfoForNextEpoch(epoch uint64) (*types.EpochInfo, error) {
	// query validator set with BLS
	queriedValSet, err := q.babylonCli.BlsPublicKeyList(epoch)
	if err != nil {
		return nil, fmt.Errorf("failed to query BLS key set for epoch %v: %w", epoch, err)
	}
	valSet := make([]*ckpttypes.ValidatorWithBlsKey, len(queriedValSet))
	for i, key := range queriedValSet {
		val := &ckpttypes.ValidatorWithBlsKey{
			ValidatorAddress: key.ValidatorAddress,
			BlsPubKey:        key.BlsPubKey,
			VotingPower:      key.VotingPower,
		}
		valSet[i] = val
	}
	// TODO should not query checkpoint here;
	// 	instead, it should be queried when verification
	//  if the checkpoint is not confirmed, buffer it
	// query checkpoint
	ckpt, err := q.babylonCli.QueryRawCheckpoint(epoch)
	if err != nil {
		return nil, fmt.Errorf("failed to query raw checkpoint of epoch %v: %w", epoch, err)
	}
	if ckpt.Ckpt.EpochNum != epoch {
		return nil, fmt.Errorf("the checkpoint is not at the desired epoch number, wanted: %v, got: %v", epoch, ckpt.Ckpt.EpochNum)
	}
	ei := types.NewEpochInfo(epoch, ckpttypes.ValidatorWithBlsKeySet{ValSet: valSet}, ckpt.Ckpt)

	// validate checkpoint
	err = ei.VerifyMultiSig(ckpt.Ckpt)
	if err != nil {
		return nil, fmt.Errorf("failed to verify Babylon checkpoint's multi-sig at epoch %v: %w", epoch, err)
	}

	return ei, nil
}

// FindTipConfirmedEpoch tries to find the last confirmed epoch number from Babylon
func (q *Querier) FindTipConfirmedEpoch() (uint64, error) {
	curEpoch, err := q.babylonCli.QueryCurrentEpoch()
	if err != nil {
		return 0, fmt.Errorf("failed to query the current epoch of Babylon: %w", err)
	}
	log.Logger.Debugf("current epoch number is %v", curEpoch)
	for curEpoch >= 1 {
		ckpt, err := q.babylonCli.QueryRawCheckpoint(curEpoch - 1)
		if err != nil {
			return 0, fmt.Errorf("failed to query the checkpoint of epoch %v: %w", curEpoch-1, err)
		}
		if ckpt.Status == ckpttypes.Confirmed || ckpt.Status == ckpttypes.Finalized {
			return curEpoch - 1, nil
		}
		curEpoch--
	}

	return 0, fmt.Errorf("cannot find a confirmed or finalized epoch from Babylon")
}

func (q *Querier) ContainsBTCHeader(hash *chainhash.Hash) (bool, error) {
	return q.babylonCli.QueryContainsBlock(hash)
}

func (q *Querier) HeaderChainTip() (*chainhash.Hash, uint64, error) {
	return q.babylonCli.QueryHeaderChainTip()
}

func (q *Querier) EndedEpochBtcHeight(epochNum uint64) (uint64, error) {
	return q.babylonCli.QueryEndedEpochBtcHeight(epochNum)
}

func (q *Querier) ReportedCheckpointBtcHeight(id string) (uint64, error) {
	return q.babylonCli.QueryReportedCheckpointBtcHeight(id)
}
