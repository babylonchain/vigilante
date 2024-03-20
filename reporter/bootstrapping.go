package reporter

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/avast/retry-go/v4"
	bbntypes "github.com/babylonchain/babylon/types"
	"github.com/babylonchain/vigilante/types"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
)

var (
	bootstrapAttempts      = uint(60)
	bootstrapAttemptsAtt   = retry.Attempts(bootstrapAttempts)
	bootstrapRetryInterval = retry.Delay(30 * time.Second)
	bootstrapDelayType     = retry.DelayType(retry.FixedDelay)
	bootstrapErrReportType = retry.LastErrorOnly(true)
)

type consistencyCheckInfo struct {
	bbnLatestBlockHeight uint64
	startSyncHeight      uint64
}

// checkConsistency checks whether the `max(bbn_tip_height - confirmation_depth, bbn_base_height)` block is same
// between BBN header chain and BTC main chain.` This makes sure that already confirmed chain is the same from point
// of view of both chains.
func (r *Reporter) checkConsistency() (*consistencyCheckInfo, error) {

	tipRes, err := r.babylonClient.BTCHeaderChainTip()
	if err != nil {
		return nil, err
	}

	// Find the base height of BBN header chain
	baseRes, err := r.babylonClient.BTCBaseHeader()
	if err != nil {
		return nil, err
	}

	var consistencyCheckHeight uint64
	if tipRes.Header.Height >= baseRes.Header.Height+r.btcConfirmationDepth {
		consistencyCheckHeight = tipRes.Header.Height - r.btcConfirmationDepth
	} else {
		consistencyCheckHeight = baseRes.Header.Height
	}

	// this checks whether header at already confirmed height is the same in reporter btc cache and in babylon btc light client
	if err := r.checkHeaderConsistency(consistencyCheckHeight); err != nil {
		return nil, err
	}

	return &consistencyCheckInfo{
		bbnLatestBlockHeight: tipRes.Header.Height,
		// we are staring from the block after already confirmed block
		startSyncHeight: consistencyCheckHeight + 1,
	}, nil
}

func (r *Reporter) bootstrap(skipBlockSubscription bool) error {
	var (
		btcLatestBlockHeight uint64
		ibs                  []*types.IndexedBlock
		err                  error
	)

	// if we are bootstraping, we will definitely not handle reorgs
	r.reorgList.clear()

	// ensure BTC has caught up with BBN header chain
	if err := r.waitUntilBTCSync(); err != nil {
		return err
	}

	// initialize cache with the latest blocks
	if err := r.initBTCCache(); err != nil {
		return err
	}
	r.logger.Debugf("BTC cache size: %d", r.btcCache.Size())

	// Subscribe new blocks right after initialising BTC cache, in order to ensure subscribed blocks and cached blocks do not have overlap.
	// Otherwise, if we subscribe too early, then they will have overlap, leading to duplicated header/ckpt submissions.
	if !skipBlockSubscription {
		r.btcClient.MustSubscribeBlocks()
	}

	consistencyInfo, err := r.checkConsistency()

	if err != nil {
		return err
	}

	ibs, err = r.btcCache.GetLastBlocks(consistencyInfo.startSyncHeight)
	if err != nil {
		panic(err)
	}

	signer := r.babylonClient.MustGetAddr()

	r.logger.Infof("BTC height: %d. BTCLightclient height: %d. Start syncing from height %d.", btcLatestBlockHeight, consistencyInfo.bbnLatestBlockHeight, consistencyInfo.startSyncHeight)

	// extracts and submits headers for each block in ibs
	// Note: As we are retrieving blocks from btc cache from block just after confirmed block which
	// we already checked for consistency, we can be sure that even if rest of the block headers is different than in Babylon
	// due to reorg, our fork will be better than the one in Babylon.
	_, err = r.ProcessHeaders(signer, ibs)
	if err != nil {
		// this can happen when there are two contentious vigilantes or if our btc node is behind.
		r.logger.Errorf("Failed to submit headers: %v", err)
		// returning error as it is up to the caller to decide what do next
		return err
	}

	// trim cache to the latest k+w blocks on BTC (which are same as in BBN)
	maxEntries := r.btcConfirmationDepth + r.checkpointFinalizationTimeout
	if err = r.btcCache.Resize(maxEntries); err != nil {
		r.logger.Errorf("Failed to resize BTC cache: %v", err)
		panic(err)
	}
	r.btcCache.Trim()

	r.logger.Infof("Size of the BTC cache: %d", r.btcCache.Size())

	// fetch k+w blocks from cache and submit checkpoints
	ibs = r.btcCache.GetAllBlocks()
	_, _, err = r.ProcessCheckpoints(signer, ibs)
	if err != nil {
		r.logger.Warnf("Failed to submit checkpoints: %v", err)
	}

	r.logger.Info("Successfully finished bootstrapping")
	return nil
}

func (r *Reporter) reporterQuitCtx() (context.Context, func()) {
	quit := r.quitChan()
	ctx, cancel := context.WithCancel(context.Background())
	r.wg.Add(1)
	go func() {
		defer cancel()
		defer r.wg.Done()

		select {
		case <-quit:

		case <-ctx.Done():
		}
	}()

	return ctx, cancel
}

func (r *Reporter) bootstrapWithRetries(skipBlockSubscription bool) {
	// if we are exiting, we need to cancel this process
	ctx, cancel := r.reporterQuitCtx()
	defer cancel()
	if err := retry.Do(func() error {
		return r.bootstrap(skipBlockSubscription)
	},
		retry.Context(ctx),
		bootstrapAttemptsAtt,
		bootstrapRetryInterval,
		bootstrapDelayType,
		bootstrapErrReportType, retry.OnRetry(func(n uint, err error) {
			r.logger.Warnf("Failed to bootstap reporter: %v. Attempt: %d, Max attempts: %d", err, n+1, bootstrapAttempts)
		})); err != nil {

		if errors.Is(err, context.Canceled) {
			// context was cancelled we do not need to anything more, app is quiting
			return
		}

		// we failed to bootstrap multiple time, we should panic as something unexpected is happening.
		r.logger.Fatalf("Failed to bootstrap reporter: %v after %d attempts", err, bootstrapAttempts)
	}
}

// initBTCCache fetches the blocks since T-k-w in the BTC canonical chain
// where T is the height of the latest block in BBN header chain
func (r *Reporter) initBTCCache() error {
	var (
		err                  error
		bbnLatestBlockHeight uint64
		bbnBaseHeight        uint64
		baseHeight           uint64
		ibs                  []*types.IndexedBlock
	)

	r.btcCache, err = types.NewBTCCache(r.Cfg.BTCCacheSize) // TODO: give an option to be unsized
	if err != nil {
		panic(err)
	}

	// get T, i.e., total block count in BBN header chain
	tipRes, err := r.babylonClient.BTCHeaderChainTip()
	if err != nil {
		return err
	}
	bbnLatestBlockHeight = tipRes.Header.Height

	// Find the base height
	baseRes, err := r.babylonClient.BTCBaseHeader()
	if err != nil {
		return err
	}
	bbnBaseHeight = baseRes.Header.Height

	// Fetch block since `baseHeight = T - k - w` from BTC, where
	// - T is total block count in BBN header chain
	// - k is btcConfirmationDepth of BBN
	// - w is checkpointFinalizationTimeout of BBN
	if bbnLatestBlockHeight > bbnBaseHeight+r.btcConfirmationDepth+r.checkpointFinalizationTimeout {
		baseHeight = bbnLatestBlockHeight - r.btcConfirmationDepth - r.checkpointFinalizationTimeout + 1
	} else {
		baseHeight = bbnBaseHeight
	}

	ibs, err = r.btcClient.FindTailBlocksByHeight(baseHeight)
	if err != nil {
		panic(err)
	}

	if err = r.btcCache.Init(ibs); err != nil {
		panic(err)
	}
	return nil
}

// waitUntilBTCSync waits for BTC to synchronize until BTC is no shorter than Babylon's BTC light client.
// It returns BTC last block hash, BTC last block height, and Babylon's base height.
func (r *Reporter) waitUntilBTCSync() error {
	var (
		btcLatestBlockHash   *chainhash.Hash
		btcLatestBlockHeight uint64
		bbnLatestBlockHash   *chainhash.Hash
		bbnLatestBlockHeight uint64
		err                  error
	)

	// Retrieve hash/height of the latest block in BTC
	btcLatestBlockHash, btcLatestBlockHeight, err = r.btcClient.GetBestBlock()
	if err != nil {
		return err
	}
	r.logger.Infof("BTC latest block hash and height: (%v, %d)", btcLatestBlockHash, btcLatestBlockHeight)

	// TODO: if BTC falls behind BTCLightclient's base header, then the vigilante is incorrectly configured and should panic

	// Retrieve hash/height of the latest block in BBN header chain
	tipRes, err := r.babylonClient.BTCHeaderChainTip()
	if err != nil {
		return err
	}

	hash, err := bbntypes.NewBTCHeaderHashBytesFromHex(tipRes.Header.HashHex)
	if err != nil {
		return err
	}

	bbnLatestBlockHash = hash.ToChainhash()
	bbnLatestBlockHeight = tipRes.Header.Height
	r.logger.Infof("BBN header chain latest block hash and height: (%v, %d)", bbnLatestBlockHash, bbnLatestBlockHeight)

	// If BTC chain is shorter than BBN header chain, pause until BTC catches up
	if btcLatestBlockHeight == 0 || btcLatestBlockHeight < bbnLatestBlockHeight {
		r.logger.Infof("BTC chain (length %d) falls behind BBN header chain (length %d), wait until BTC catches up", btcLatestBlockHeight, bbnLatestBlockHeight)

		// periodically check if BTC catches up with BBN.
		// When BTC catches up, break and continue the bootstrapping process
		ticker := time.NewTicker(5 * time.Second) // TODO: parameterise the polling interval
		for range ticker.C {
			_, btcLatestBlockHeight, err = r.btcClient.GetBestBlock()
			if err != nil {
				return err
			}
			tipRes, err = r.babylonClient.BTCHeaderChainTip()
			if err != nil {
				return err
			}
			bbnLatestBlockHeight = tipRes.Header.Height
			if btcLatestBlockHeight > 0 && btcLatestBlockHeight >= bbnLatestBlockHeight {
				r.logger.Infof("BTC chain (length %d) now catches up with BBN header chain (length %d), continue bootstrapping", btcLatestBlockHeight, bbnLatestBlockHeight)
				break
			}
			r.logger.Infof("BTC chain (length %d) still falls behind BBN header chain (length %d), keep waiting", btcLatestBlockHeight, bbnLatestBlockHeight)
		}
	}

	return nil
}

func (r *Reporter) checkHeaderConsistency(consistencyCheckHeight uint64) error {
	var err error

	consistencyCheckBlock := r.btcCache.FindBlock(consistencyCheckHeight)
	if consistencyCheckBlock == nil {
		err = fmt.Errorf("cannot find the %d-th block of BBN header chain in BTC cache for initial consistency check", consistencyCheckHeight)
		panic(err)
	}
	consistencyCheckHash := consistencyCheckBlock.BlockHash()

	r.logger.Debugf("block for consistency check: height %d, hash %v", consistencyCheckHeight, consistencyCheckHash)

	// Given that two consecutive BTC headers are chained via hash functions,
	// generating a header that can be in two different positions in two different BTC header chains
	// is as hard as breaking the hash function.
	// So as long as the block exists on Babylon, it has to be at the same position as in Babylon as well.
	res, err := r.babylonClient.ContainsBTCBlock(&consistencyCheckHash) // TODO: this API has error. Find out why
	if err != nil {
		return err
	}
	if !res.Contains {
		err = fmt.Errorf("BTC main chain is inconsistent with BBN header chain: k-deep block in BBN header chain: %v", consistencyCheckHash)
		panic(err)
	}
	return nil
}
