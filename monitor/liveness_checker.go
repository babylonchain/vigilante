package monitor

import (
	"fmt"
	"time"

	monitortypes "github.com/babylonchain/babylon/x/monitor/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	"github.com/babylonchain/vigilante/types"
)

func (m *Monitor) LivenessChecker() {
	ticker := time.NewTicker(time.Duration(m.Cfg.LivenessCheckIntervalSeconds) * time.Second)
	log.Infof("liveness checker is started, checking liveness every %d seconds", m.Cfg.LivenessCheckIntervalSeconds)

	for {
		select {
		case <-ticker.C:
			log.Debugf("next liveness check is in %d seconds", m.Cfg.LivenessCheckIntervalSeconds)
			checkpoints := m.checkpointChecklist.GetAll()
			for _, c := range checkpoints {
				err := m.CheckLiveness(c)
				if err != nil {
					// TODO decide what to do with this error, sending an alarm?
					panic(fmt.Errorf("the checkpoint %x at epoch %v is detected being censored: %w", c.ID(), c.EpochNum(), err))
				}
			}
		}
	}
}

// CheckLiveness checks whether the Babylon node is under liveness attack with the following steps
// 1. ask Babylon the BTC light client height when the epoch ends (H1)
// 2. (denote c.firstBtcHeight as H2, which is the BTC height at which the unique checkpoint first appears)
// 3. ask Babylon the tip height of BTC light client when the checkpoint is reported (H3)
// 4. if the checkpoint is not reported, ask Babylon the current tip height of BTC light client (H4)
// 5. if H3 - min(H1, H2) > max_live_btc_heights (if the checkpoint is reported), or
//    H4 - min(H1, H2) > max_live_btc_heights (if the checkpoint is not reported), return error
func (m *Monitor) CheckLiveness(cr *types.CheckpointRecord) error {
	var (
		h1  uint64 // the BTC light client height when the epoch ends (obtained from Babylon)
		h2  uint64 // the BTC height at which the unique checkpoint first appears (obtained from BTC)
		h3  uint64 // the tip height of BTC light client when the checkpoint is reported (obtained from Babylon)
		h4  uint64 // the current tip height of BTC light client (obtained from Babylon)
		gap uint64 // the gap between two BTC heights
		err error
	)
	epoch := cr.EpochNum()
	h1, err = m.Querier.FinishedEpochBtcHeight(cr.EpochNum())
	if err != nil {
		return fmt.Errorf("the checkpoint at epoch %d is submitted on BTC the epoch is not finished on Babylon: %w", epoch, err)
	}

	h2 = cr.FirstSeenBtcHeight
	minHeight := minBTCHeight(h1, h2)

	h3, err = m.Querier.ReportedCheckpointBtcHeight(cr.ID())
	if err != nil {
		if sdkerrors.IsOf(monitortypes.ErrCheckpointNotReported) {
			h4, err = m.Querier.TipBTCHeight()
			if err != nil {
				return err
			}
			gap = h4 - minHeight
		}
		gap = h3 - minHeight
	}

	if gap > m.Cfg.MaxLiveBtcHeights {
		return fmt.Errorf("%w: the gap BTC height is %d, larger than the threshold %d", types.ErrLivenessAttack, gap, m.Cfg.MaxLiveBtcHeights)
	}

	return nil
}
