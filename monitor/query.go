package monitor

import (
	"fmt"

	"github.com/babylonchain/babylon/types/retry"
	btclctypes "github.com/babylonchain/babylon/x/btclightclient/types"
	ckpttypes "github.com/babylonchain/babylon/x/checkpointing/types"
	epochingtypes "github.com/babylonchain/babylon/x/epoching/types"
	monitortypes "github.com/babylonchain/babylon/x/monitor/types"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"go.uber.org/zap"

	"github.com/babylonchain/vigilante/types"
)

// QueryInfoForNextEpoch fetches necessary information for verifying the next epoch from Babylon
func (m *Monitor) QueryInfoForNextEpoch(epoch uint64) (*types.EpochInfo, error) {
	// query validator set with BLS
	res, err := m.queryBlsPublicKeyListWithRetry(epoch)
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
	epochRes, err := m.queryCurrentEpochWithRetry()
	if err != nil {
		return 0, fmt.Errorf("failed to query the current epoch of Babylon: %w", err)
	}
	curEpoch := epochRes.CurrentEpoch
	m.logger.Debugf("current epoch number is %v", curEpoch)
	for curEpoch >= 1 {
		ckptRes, err := m.queryRawCheckpointWithRetry(curEpoch - 1)
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

func (m *Monitor) queryCurrentEpochWithRetry() (*epochingtypes.QueryCurrentEpochResponse, error) {
	var currentEpochRes epochingtypes.QueryCurrentEpochResponse

	if err := retry.Do(m.ComCfg.RetrySleepTime, m.ComCfg.MaxRetrySleepTime, func() error {
		res, err := m.BBNQuerier.CurrentEpoch()
		if err != nil {
			return err
		}

		currentEpochRes = *res
		return nil
	}); err != nil {
		m.logger.Debug(
			"failed to query the current epoch", zap.Error(err))

		return nil, err
	}

	return &currentEpochRes, nil
}

func (m *Monitor) queryRawCheckpointWithRetry(epoch uint64) (*ckpttypes.QueryRawCheckpointResponse, error) {
	var rawCheckpointRes ckpttypes.QueryRawCheckpointResponse

	if err := retry.Do(m.ComCfg.RetrySleepTime, m.ComCfg.MaxRetrySleepTime, func() error {
		res, err := m.BBNQuerier.RawCheckpoint(epoch)
		if err != nil {
			return err
		}

		rawCheckpointRes = *res
		return nil
	}); err != nil {
		m.logger.Debug(
			"failed to query the raw checkpoint", zap.Error(err))

		return nil, err
	}

	return &rawCheckpointRes, nil
}

func (m *Monitor) queryBlsPublicKeyListWithRetry(epoch uint64) (*ckpttypes.QueryBlsPublicKeyListResponse, error) {
	var blsPublicKeyListRes ckpttypes.QueryBlsPublicKeyListResponse

	if err := retry.Do(m.ComCfg.RetrySleepTime, m.ComCfg.MaxRetrySleepTime, func() error {
		res, err := m.BBNQuerier.BlsPublicKeyList(epoch, nil)
		if err != nil {
			return err
		}

		blsPublicKeyListRes = *res
		return nil
	}); err != nil {
		m.logger.Debug(
			"failed to query the BLS public key list", zap.Error(err))

		return nil, err
	}

	return &blsPublicKeyListRes, nil
}

func (m *Monitor) queryEndedEpochBTCHeightWithRetry(epoch uint64) (*monitortypes.QueryEndedEpochBtcHeightResponse, error) {
	var endedEpochBTCHeightRes monitortypes.QueryEndedEpochBtcHeightResponse

	if err := retry.Do(m.ComCfg.RetrySleepTime, m.ComCfg.MaxRetrySleepTime, func() error {
		res, err := m.BBNQuerier.EndedEpochBTCHeight(epoch)
		if err != nil {
			return err
		}

		endedEpochBTCHeightRes = *res
		return nil
	}); err != nil {
		m.logger.Debug(
			"failed to query the ended epoch BTC height", zap.Error(err))

		return nil, err
	}

	return &endedEpochBTCHeightRes, nil
}

func (m *Monitor) queryReportedCheckpointBTCHeightWithRetry(hashStr string) (*monitortypes.QueryReportedCheckpointBtcHeightResponse, error) {
	var reportedCheckpointBtcHeightRes monitortypes.QueryReportedCheckpointBtcHeightResponse

	if err := retry.Do(m.ComCfg.RetrySleepTime, m.ComCfg.MaxRetrySleepTime, func() error {
		res, err := m.BBNQuerier.ReportedCheckpointBTCHeight(hashStr)
		if err != nil {
			return err
		}

		reportedCheckpointBtcHeightRes = *res
		return nil
	}); err != nil {
		m.logger.Debug(
			"failed to query the reported checkpoint BTC height", zap.Error(err))

		return nil, err
	}

	return &reportedCheckpointBtcHeightRes, nil
}

func (m *Monitor) queryBTCHeaderChainTipWithRetry() (*btclctypes.QueryTipResponse, error) {
	var btcHeaderChainTipRes btclctypes.QueryTipResponse

	if err := retry.Do(m.ComCfg.RetrySleepTime, m.ComCfg.MaxRetrySleepTime, func() error {
		res, err := m.BBNQuerier.BTCHeaderChainTip()
		if err != nil {
			return err
		}

		btcHeaderChainTipRes = *res
		return nil
	}); err != nil {
		m.logger.Debug(
			"failed to query the BTC header chain tip", zap.Error(err))

		return nil, err
	}

	return &btcHeaderChainTipRes, nil
}

func (m *Monitor) queryContainsBTCBlockWithRetry(blockHash *chainhash.Hash) (*btclctypes.QueryContainsBytesResponse, error) {
	var containsBTCBlockRes btclctypes.QueryContainsBytesResponse

	if err := retry.Do(m.ComCfg.RetrySleepTime, m.ComCfg.MaxRetrySleepTime, func() error {
		res, err := m.BBNQuerier.ContainsBTCBlock(blockHash)
		if err != nil {
			return err
		}

		containsBTCBlockRes = *res
		return nil
	}); err != nil {
		m.logger.Debug(
			"failed to query the contains BTC block", zap.Error(err))

		return nil, err
	}

	return &containsBTCBlockRes, nil
}
