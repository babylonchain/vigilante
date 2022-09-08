package reporter

import (
	"time"

	"github.com/babylonchain/vigilante/types"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
)

func (r *Reporter) Init() {
	var (
		btcLatestBlockHash   *chainhash.Hash
		btcLatestBlockHeight int32
		bbnLatestBlockHash   *chainhash.Hash
		bbnLatestBlockHeight uint64
		err                  error
	)

	// retrieve k and w within btccParams
	btccParams, err := r.babylonClient.QueryBTCCheckpointParams()
	if err != nil {
		panic(err)
	}
	btcConfirmationDepth := btccParams.BtcConfirmationDepth                   // k
	checkpointFinalizationTimeout := btccParams.CheckpointFinalizationTimeout // w
	log.Infof("BTCCheckpoint parameters: (k, w) = (%d, %d)", btcConfirmationDepth, checkpointFinalizationTimeout)

	// Download h-w blocks and initialize BTC Cache
	if err = r.initBTCCache(btcConfirmationDepth, checkpointFinalizationTimeout); err != nil {
		panic(err)
	}

	// retrieve hash/height of the latest block in BTC
	btcLatestBlockHash, btcLatestBlockHeight, err = r.btcClient.GetBestBlock()
	if err != nil {
		panic(err)
	}
	log.Infof("BTC latest block hash and height: (%v, %d)", btcLatestBlockHash, btcLatestBlockHeight)

	// TODO: if BTC falls behind BTCLightclient's base header, then the vigilante is incorrectly configured and should panic

	// retrieve hash/height of the latest block in BBN header chain
	bbnLatestBlockHash, bbnLatestBlockHeight, err = r.babylonClient.QueryHeaderChainTip()
	if err != nil {
		panic(err)
	}
	log.Infof("BBN header chain latest block hash and height: (%v, %d)", bbnLatestBlockHash, bbnLatestBlockHeight)

	// if BTC chain is shorter than BBN header chain, pause until BTC catches up
	if uint64(btcLatestBlockHeight) < bbnLatestBlockHeight {
		log.Infof("BTC chain (length %d) falls behind BBN header chain (length %d), wait until BTC catches up", btcLatestBlockHeight, bbnLatestBlockHeight)

		// periodically check if BTC catches up with BBN.
		// When BTC catches up, break and continue the bootstrapping process
		ticker := time.NewTicker(5 * time.Second) // TODO: parameterise the polling interval
		for range ticker.C {
			btcLatestBlockHash, btcLatestBlockHeight, err = r.btcClient.GetBestBlock()
			if err != nil {
				panic(err)
			}
			bbnLatestBlockHash, bbnLatestBlockHeight, err = r.babylonClient.QueryHeaderChainTip()
			if err != nil {
				panic(err)
			}
			if uint64(btcLatestBlockHeight) >= bbnLatestBlockHeight {
				log.Infof("BTC chain (length %d) now catches up with BBN header chain (length %d), continue bootstrapping", btcLatestBlockHeight, bbnLatestBlockHeight)
				break
			}
			log.Infof("BTC chain (length %d) still falls behind BBN header chain (length %d), keep waiting", btcLatestBlockHeight, bbnLatestBlockHeight)
		}
	} else if uint64(btcLatestBlockHeight) == bbnLatestBlockHeight {
		// TODO: initial consistency check
	} else {
		// Extract headers from BTC cache and forward them to BBN
		ibs := r.btcCache.GetLastBlocks(bbnLatestBlockHeight)

		signer := r.babylonClient.MustGetAddr()
		for _, ib := range ibs {
			blockHash := ib.BlockHash()
			if err = r.submitHeader(signer, ib.Header); err != nil {
				log.Errorf("Failed to handle header %v from Bitcoin: %v", blockHash, err)
			}
		}
	}

	// TODO: extract ckpt segments from BTC cache, store them in ckpt segment pool, check newly matched ckpts, and forward newly matched ckpts to BBN
}

// initBTCCache fetches the last blocks in the BTC canonical chain
// TODO: make the BTC cache size a system parameter
func (r *Reporter) initBTCCache(btcConfirmationDepth, checkpointFinalizationTimeout uint64) error {
	var (
		err             error
		totalBlockCount int64
		ibs             []*types.IndexedBlock
		btcCache        = r.btcCache
	)

	totalBlockCount, err = r.btcClient.GetBlockCount()
	if err != nil {
		return err
	}

	// Fetch h - w blocks where h is height of K deep block
	kDeepBlockHeight := uint64(totalBlockCount) - btcConfirmationDepth
	stopHeight := kDeepBlockHeight - checkpointFinalizationTimeout
	ibs, err = r.btcClient.GetLastBlocks(stopHeight)
	if err != nil {
		return err
	}

	return btcCache.Init(ibs)
}
