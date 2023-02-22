package monitor_test

import (
	"testing"

	bbn_datagen "github.com/babylonchain/babylon/testutil/datagen"
	monitortypes "github.com/babylonchain/babylon/x/monitor/types"
	"github.com/babylonchain/rpc-client/testutil/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/babylonchain/vigilante/config"
	"github.com/babylonchain/vigilante/monitor"
	"github.com/babylonchain/vigilante/querier"
	"github.com/babylonchain/vigilante/testutil/datagen"
	"github.com/babylonchain/vigilante/types"
)

func FuzzLivenessChecker(f *testing.F) {
	bbn_datagen.AddRandomSeedsToFuzzer(f, 100)

	f.Fuzz(func(t *testing.T, seed int64) {
		ctl := gomock.NewController(t)
		mockBabylonClient := mocks.NewMockBabylonQueryClient(ctl)
		q := querier.New(mockBabylonClient)
		cr := datagen.GenerateRandomCheckpointRecord()
		maxGap := bbn_datagen.RandomIntOtherThan(0, 50) + 200
		cfg := &config.MonitorConfig{MaxLiveBtcHeights: maxGap}
		m := &monitor.Monitor{
			Cfg:        cfg,
			BBNQuerier: q,
		}

		// 1. normal case, checkpoint is reported, h1 < h2 < h3, h3 - h1 < MaxLiveBtcHeights
		h1 := bbn_datagen.RandomIntOtherThan(0, 50)
		h2 := bbn_datagen.RandomIntOtherThan(0, 50) + h1
		cr.FirstSeenBtcHeight = h2
		h3 := bbn_datagen.RandomIntOtherThan(0, 50) + h2
		mockBabylonClient.EXPECT().EndedEpochBTCHeight(gomock.Eq(cr.EpochNum())).Return(h1, nil).AnyTimes()
		mockBabylonClient.EXPECT().ReportedCheckpointBTCHeight(gomock.Eq(cr.ID())).Return(h3, nil)
		err := m.CheckLiveness(cr)
		require.NoError(t, err)

		// 2. attack case, checkpoint is reported, h1 < h2 < h3, h3 - h1 > MaxLiveBtcHeights
		h3 = bbn_datagen.RandomIntOtherThan(0, 50) + h2 + maxGap
		mockBabylonClient.EXPECT().ReportedCheckpointBTCHeight(gomock.Eq(cr.ID())).Return(h3, nil)
		err = m.CheckLiveness(cr)
		require.ErrorIs(t, err, types.ErrLivenessAttack)

		// 3. normal case, checkpoint is not reported, h1 < h2 < h4, h4 - h1 < MaxLiveBtcHeights
		h4 := bbn_datagen.RandomIntOtherThan(0, 50) + h2
		mockBabylonClient.EXPECT().ReportedCheckpointBTCHeight(gomock.Eq(cr.ID())).Return(uint64(0), monitortypes.ErrCheckpointNotReported)
		mockBabylonClient.EXPECT().BTCHeaderChainTip().Return(nil, h4, nil)
		err = m.CheckLiveness(cr)
		require.NoError(t, err)

		// 4. attack case, checkpoint is not reported, h1 < h2 < h4, h4 - h1 > MaxLiveBtcHeights
		h4 = bbn_datagen.RandomIntOtherThan(0, 50) + h2 + maxGap
		mockBabylonClient.EXPECT().ReportedCheckpointBTCHeight(gomock.Eq(cr.ID())).Return(uint64(0), monitortypes.ErrCheckpointNotReported)
		mockBabylonClient.EXPECT().BTCHeaderChainTip().Return(nil, h4, nil)
		err = m.CheckLiveness(cr)
		require.ErrorIs(t, err, types.ErrLivenessAttack)
	})
}
