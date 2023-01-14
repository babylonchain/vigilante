package monitor

import (
	"fmt"
	"time"

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
				err := m.checkLiveness(c)
				if err != nil {
					// TODO decide what to do with this error, sending an alarm?
					panic(fmt.Errorf("the checkpoint %x at epoch %v is detected being censored: %w", c.ID(), c.EpochNum(), err))
				}
			}
		}
	}
}

// checkLiveness checks whether the Babylon node is under liveness attack with the following steps
// 1. ask Babylon the BTC light client height when the epoch ends (H1)
// 2. (denote c.firstBtcHeight as H2, which is the BTC height at which the unique checkpoint first appears)
// 3. ask Babylon the tip height of BTC light client when the checkpoint is reported (H3)
// 4. if the checkpoint is not reported, ask Babylon the current tip height of BTC light client (H4)
// 5. if H3 - min(H1, H2) > max_live_btc_heights (if the checkpoint is reported), or
//    H4 - min(H1, H2) > max_live_btc_heights (if the checkpoint is not reported), return error
func (m *Monitor) checkLiveness(c *types.CheckpointRecord) error {
	// TODO implement relevant Babylon APIs
	return nil
}
