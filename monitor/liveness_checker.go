package monitor

import (
	"fmt"
	"time"

	"github.com/pkg/errors"

	monitortypes "github.com/babylonchain/babylon/x/monitor/types"

	"github.com/babylonchain/vigilante/types"
)

func (m *Monitor) runLivenessChecker() {
	ticker := time.NewTicker(time.Duration(m.Cfg.LivenessCheckIntervalSeconds) * time.Second)

	log.Infof("liveness checker is started, checking liveness every %d seconds", m.Cfg.LivenessCheckIntervalSeconds)

	for m.started.Load() {
		select {
		case <-m.quit:
			m.wg.Done()
			m.started.Store(false)
		case <-ticker.C:
			log.Debugf("next liveness check is in %d seconds", m.Cfg.LivenessCheckIntervalSeconds)
			checkpoints := m.checkpointChecklist.GetAll()
			for _, c := range checkpoints {
				err := m.CheckLiveness(c)
				if err != nil {
					// TODO decide what to do with this error, sending an alarm?
					log.Errorf("the checkpoint at epoch %d is detected being censored: %s", c.EpochNum(), err.Error())
					continue
				}
				log.Debugf("the checkpoint at epoch %d has passed the liveness check", c.EpochNum())
				m.checkpointChecklist.Remove(c.ID())
			}
		}
	}

	log.Info("the liveness checker is stopped")
}

// CheckLiveness checks whether the Babylon node is under liveness attack with the following steps
//  1. ask Babylon the BTC light client height when the epoch ends (H1)
//  2. (denote c.firstBtcHeight as H2, which is the BTC height at which the unique checkpoint first appears)
//  3. ask Babylon the tip height of BTC light client when the checkpoint is reported (H3)
//  4. if the checkpoint is not reported, ask Babylon the current tip height of BTC light client (H4)
//  5. if H3 - min(H1, H2) > max_live_btc_heights (if the checkpoint is reported), or
//     H4 - min(H1, H2) > max_live_btc_heights (if the checkpoint is not reported), return error
func (m *Monitor) CheckLiveness(cr *types.CheckpointRecord) error {
	var (
		btcHeightEpochEnded   uint64 // the BTC light client height when the epoch ends (obtained from Babylon)
		btcHeightFirstSeen    uint64 // the BTC height at which the unique checkpoint first appears (obtained from BTC)
		btcHeightCkptReported uint64 // the tip height of BTC light client when the checkpoint is reported (obtained from Babylon)
		currentBtcTipHeight   uint64 // the current tip height of BTC light client (obtained from Babylon)
		gap                   int    // the gap between two BTC heights
		err                   error
	)
	epoch := cr.EpochNum()
	endedEpochRes, err := m.BBNQuerier.EndedEpochBTCHeight(cr.EpochNum())
	if err != nil {
		return fmt.Errorf("the checkpoint at epoch %d is submitted on BTC the epoch is not ended on Babylon: %w", epoch, err)
	}
	btcHeightEpochEnded = endedEpochRes.BtcLightClientHeight
	log.Debugf("the epoch %d is ended at BTC height %d", cr.EpochNum(), btcHeightEpochEnded)

	btcHeightFirstSeen = cr.FirstSeenBtcHeight
	minHeight := minBTCHeight(btcHeightEpochEnded, btcHeightFirstSeen)

	reportedRes, err := m.BBNQuerier.ReportedCheckpointBTCHeight(cr.ID())
	btcHeightCkptReported = reportedRes.BtcLightClientHeight
	if err != nil {
		if !errors.Is(err, monitortypes.ErrCheckpointNotReported) {
			return fmt.Errorf("failed to query checkpoint of epoch %d reported BTC height: %w", epoch, err)
		}
		log.Debugf("the checkpoint of epoch %d has not been reported: %s", epoch, err.Error())
		chainTipRes, err := m.BBNQuerier.BTCHeaderChainTip()
		if err != nil {
			return fmt.Errorf("failed to query the current tip height of BTC light client: %w", err)
		}
		currentBtcTipHeight = chainTipRes.Header.Height
		log.Debugf("the current tip height of BTC light client is %d", currentBtcTipHeight)
		gap = int(currentBtcTipHeight) - int(minHeight)
	} else {
		gap = int(btcHeightCkptReported) - int(minHeight)
	}

	if gap < 0 {
		return fmt.Errorf("the gap %d between two BTC heights should not be negative", gap)
	}

	if gap > int(m.Cfg.MaxLiveBtcHeights) {
		return fmt.Errorf("%w: the gap BTC height is %d, larger than the threshold %d", types.ErrLivenessAttack, gap, m.Cfg.MaxLiveBtcHeights)
	}

	return nil
}
