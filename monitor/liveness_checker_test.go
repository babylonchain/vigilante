package monitor_test

import (
	datagen2 "github.com/babylonchain/babylon/testutil/datagen"
	monitortypes "github.com/babylonchain/babylon/x/monitor/types"
	"github.com/babylonchain/rpc-client/testutil/mocks"
	"github.com/babylonchain/vigilante/config"
	"github.com/babylonchain/vigilante/monitor"
	"github.com/babylonchain/vigilante/monitor/querier"
	"github.com/babylonchain/vigilante/testutil/datagen"
	"github.com/babylonchain/vigilante/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"testing"
)

func FuzzLivenessChecker(f *testing.F) {
	datagen2.AddRandomSeedsToFuzzer(f, 100)

	f.Fuzz(func(t *testing.T, seed int64) {
		ctl := gomock.NewController(t)
		mockBabylonClient := mocks.NewMockBabylonClient(ctl)
		q := &querier.Querier{BabylonClient: mockBabylonClient}
		cr := datagen.GenerateRandomCheckpointRecord()
		cfg := &config.MonitorConfig{MaxLiveBtcHeights: 200}
		m := &monitor.Monitor{
			Cfg:     cfg,
			Querier: q,
		}

		// 1. normal case, checkpoint is reported, h1 < h2 < h3, h3 - h1 < MaxLiveBtcHeights
		h1 := datagen2.RandomIntOtherThan(0, 50)
		h2 := datagen2.RandomIntOtherThan(0, 50) + h1
		cr.FirstSeenBtcHeight = h2
		h3 := datagen2.RandomIntOtherThan(0, 50) + h2
		mockBabylonClient.EXPECT().QueryFinishedEpochBtcHeight(gomock.Eq(cr.EpochNum())).Return(h1, nil).AnyTimes()
		mockBabylonClient.EXPECT().QueryReportedCheckpointBtcHeight(gomock.Eq(cr.ID())).Return(h3, nil)
		err := m.CheckLiveness(cr)
		require.NoError(t, err)

		// 2. attack case, checkpoint is reported, h1 < h2 < h3, h3 - h1 > MaxLiveBtcHeights
		h3 = datagen2.RandomIntOtherThan(0, 50) + h2 + 200
		mockBabylonClient.EXPECT().QueryReportedCheckpointBtcHeight(gomock.Eq(cr.ID())).Return(h3, nil)
		err = m.CheckLiveness(cr)
		require.ErrorIs(t, err, types.ErrLivenessAttack)

		// 3. normal case, checkpoint is not reported, h1 < h2 < h4, h4 - h1 < MaxLiveBtcHeights
		h4 := datagen2.RandomIntOtherThan(0, 50) + h2
		mockBabylonClient.EXPECT().QueryReportedCheckpointBtcHeight(gomock.Eq(cr.ID())).Return(uint64(0), monitortypes.ErrCheckpointNotReported)
		mockBabylonClient.EXPECT().QueryHeaderChainTip().Return(nil, h4, nil)
		err = m.CheckLiveness(cr)
		require.NoError(t, err)

		// 4. attack case, checkpoint is not reported, h1 < h2 < h4, h4 - h1 > MaxLiveBtcHeights
		h4 = datagen2.RandomIntOtherThan(0, 50) + h2 + 200
		mockBabylonClient.EXPECT().QueryReportedCheckpointBtcHeight(gomock.Eq(cr.ID())).Return(uint64(0), monitortypes.ErrCheckpointNotReported)
		mockBabylonClient.EXPECT().QueryHeaderChainTip().Return(nil, h4, nil)
		err = m.CheckLiveness(cr)
		require.ErrorIs(t, err, types.ErrLivenessAttack)
	})
}
