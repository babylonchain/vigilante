package monitor

import (
	"fmt"

	ckpttypes "github.com/babylonchain/babylon/x/checkpointing/types"

	"github.com/babylonchain/vigilante/types"
)

// QueryInfoForNextEpoch fetches necessary information for verifying the next epoch from Babylon
func (m *Monitor) QueryInfoForNextEpoch(epoch uint64) (*types.EpochInfo, error) {
	// query validator set with BLS
	res, err := m.BBNQuerier.BlsPublicKeyList(epoch, nil)
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
func (m *Monitor) FindTipConfirmedEpoch() (uint64, error) {
	epochRes, err := m.BBNQuerier.CurrentEpoch()
	if err != nil {
		return 0, fmt.Errorf("failed to query the current epoch of Babylon: %w", err)
	}
	curEpoch := epochRes.CurrentEpoch
	m.logger.Debugf("current epoch number is %v", curEpoch)
	for curEpoch >= 1 {
		ckptRes, err := m.BBNQuerier.RawCheckpoint(curEpoch - 1)
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
