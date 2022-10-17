package poller_test

import (
	checkpointingtypes "github.com/babylonchain/babylon/x/checkpointing/types"
	"github.com/babylonchain/vigilante/submitter/poller"
	"github.com/babylonchain/vigilante/testutil/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"math/rand"
	"sort"
	"sync"
	"testing"

	"github.com/babylonchain/babylon/testutil/datagen"
)

func FuzzPollingCheckpoints(f *testing.F) {
	/*
		Checks:
		- the poller polls Sealed checkpoints,
		only the oldest one being pushed into the channel

		Data generation:
		- a series of raw checkpoints
	*/
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		var wg sync.WaitGroup
		rand.Seed(seed)
		n := rand.Intn(10) + 1
		sealedCkpts := make([]*checkpointingtypes.RawCheckpointWithMeta, n)
		for i := 0; i < n; i++ {
			ckpt := datagen.GenRandomRawCheckpointWithMeta()
			ckpt.Status = checkpointingtypes.Sealed
			sealedCkpts[i] = ckpt
		}
		sort.Slice(sealedCkpts, func(i, j int) bool {
			return sealedCkpts[i].Ckpt.EpochNum < sealedCkpts[j].Ckpt.EpochNum
		})
		bbnClient := mocks.NewMockBabylonClient(gomock.NewController(t))
		bbnClient.EXPECT().QueryRawCheckpointList(checkpointingtypes.Sealed).Return(sealedCkpts, nil)
		testPoller := poller.New(bbnClient, 100)
		wg.Add(1)
		var ckpt *checkpointingtypes.RawCheckpointWithMeta
		go func() {
			defer wg.Done()
			ckpt = <-testPoller.GetSealedCheckpointChan()
		}()
		err := testPoller.PollSealedCheckpoints()
		wg.Wait()
		require.NoError(t, err)
		require.True(t, sealedCkpts[0].Equal(ckpt))
	})
}
