package btcscanner

import (
	"fmt"
	"sync"

	"github.com/babylonchain/babylon/btctxformatter"
	ckpttypes "github.com/babylonchain/babylon/x/checkpointing/types"
	"github.com/btcsuite/btcd/wire"
	"go.uber.org/atomic"
	"go.uber.org/zap"

	"github.com/babylonchain/vigilante/btcclient"
	"github.com/babylonchain/vigilante/config"
	"github.com/babylonchain/vigilante/types"
)

type BtcScanner struct {
	logger *zap.SugaredLogger

	// connect to BTC node
	BtcClient btcclient.BTCClient

	// the BTC height the scanner starts
	BaseHeight uint64
	// the BTC confirmation depth
	K uint64

	confirmedTipBlock   *types.IndexedBlock
	ConfirmedBlocksChan chan *types.IndexedBlock

	// cache of a sequence of checkpoints
	ckptCache *types.CheckpointCache
	// cache of a sequence of unconfirmed blocks
	UnconfirmedBlockCache *types.BTCCache

	// communicate with the monitor
	blockHeaderChan chan *wire.BlockHeader
	checkpointsChan chan *types.CheckpointRecord

	Synced *atomic.Bool

	wg      sync.WaitGroup
	Started *atomic.Bool
	quit    chan struct{}
}

func New(
	monitorCfg *config.MonitorConfig,
	parentLogger *zap.Logger,
	btcClient btcclient.BTCClient,
	btclightclientBaseHeight uint64,
	checkpointTag []byte,
) (*BtcScanner, error) {
	headersChan := make(chan *wire.BlockHeader, monitorCfg.BtcBlockBufferSize)
	confirmedBlocksChan := make(chan *types.IndexedBlock, monitorCfg.BtcBlockBufferSize)
	ckptsChan := make(chan *types.CheckpointRecord, monitorCfg.CheckpointBufferSize)
	ckptCache := types.NewCheckpointCache(checkpointTag, btctxformatter.CurrentVersion)
	unconfirmedBlockCache, err := types.NewBTCCache(monitorCfg.BtcCacheSize)
	if err != nil {
		panic(fmt.Errorf("failed to create BTC cache for tail blocks: %w", err))
	}

	return &BtcScanner{
		logger:                parentLogger.With(zap.String("module", "btcscanner")).Sugar(),
		BtcClient:             btcClient,
		BaseHeight:            btclightclientBaseHeight,
		K:                     monitorCfg.BtcConfirmationDepth,
		ckptCache:             ckptCache,
		UnconfirmedBlockCache: unconfirmedBlockCache,
		ConfirmedBlocksChan:   confirmedBlocksChan,
		blockHeaderChan:       headersChan,
		checkpointsChan:       ckptsChan,
		Synced:                atomic.NewBool(false),
		Started:               atomic.NewBool(false),
		quit:                  make(chan struct{}),
	}, nil
}

// Start starts the scanning process from curBTCHeight to tipHeight
func (bs *BtcScanner) Start() {
	if bs.Started.Load() {
		bs.logger.Info("the BTC scanner is already started")
		return
	}

	// the bootstrapping should not block the main thread
	go bs.Bootstrap()

	bs.BtcClient.MustSubscribeBlocks()

	bs.Started.Store(true)
	bs.logger.Info("the BTC scanner is started")

	// start handling new blocks
	bs.wg.Add(1)
	go bs.blockEventHandler()

	for bs.Started.Load() {
		select {
		case <-bs.quit:
			bs.Started.Store(false)
		case block := <-bs.ConfirmedBlocksChan:
			bs.logger.Debugf("found a confirmed BTC block at height %d", block.Height)
			// send the header to the Monitor for consistency check
			bs.blockHeaderChan <- block.Header
			ckptBtc := bs.tryToExtractCheckpoint(block)
			if ckptBtc == nil {
				bs.logger.Debugf("checkpoint not found at BTC block %v", block.Height)
				// move to the next BTC block
				continue
			}
			bs.logger.Infof("found a checkpoint at BTC block %d", ckptBtc.FirstSeenBtcHeight)

			bs.checkpointsChan <- ckptBtc
		}
	}

	bs.wg.Wait()
	bs.logger.Info("the BTC scanner is stopped")
}

// Bootstrap syncs with BTC by getting the confirmed blocks and the caching the unconfirmed blocks
func (bs *BtcScanner) Bootstrap() {
	var (
		firstUnconfirmedHeight uint64
		confirmedBlock         *types.IndexedBlock
		err                    error
	)

	if bs.Synced.Load() {
		// the scanner is already synced
		return
	}
	defer bs.Synced.Store(true)

	if bs.confirmedTipBlock != nil {
		firstUnconfirmedHeight = uint64(bs.confirmedTipBlock.Height + 1)
	} else {
		firstUnconfirmedHeight = bs.BaseHeight
	}

	bs.logger.Infof("the bootstrapping starts at %d", firstUnconfirmedHeight)

	// clear all the blocks in the cache to avoid forks
	bs.UnconfirmedBlockCache.RemoveAll()

	_, bestHeight, err := bs.BtcClient.GetBestBlock()
	if err != nil {
		panic(fmt.Errorf("cannot get the best BTC block"))
	}

	bestConfirmedHeight := bestHeight - bs.K
	// process confirmed blocks
	for i := firstUnconfirmedHeight; i <= bestConfirmedHeight; i++ {
		ib, _, err := bs.BtcClient.GetBlockByHeight(i)
		if err != nil {
			panic(err)
		}

		// this is a confirmed block
		confirmedBlock = ib

		// if the scanner was bootstrapped before, the new confirmed canonical chain must connect to the previous one
		if bs.confirmedTipBlock != nil {
			confirmedTipHash := bs.confirmedTipBlock.BlockHash()
			if !confirmedTipHash.IsEqual(&confirmedBlock.Header.PrevBlock) {
				panic("invalid canonical chain")
			}
		}

		bs.sendConfirmedBlocksToChan([]*types.IndexedBlock{confirmedBlock})
	}

	// add unconfirmed blocks into the cache
	for i := bestConfirmedHeight + 1; i <= bestHeight; i++ {
		ib, _, err := bs.BtcClient.GetBlockByHeight(i)
		if err != nil {
			panic(err)
		}

		// the unconfirmed blocks must follow the canonical chain
		tipCache := bs.UnconfirmedBlockCache.Tip()
		if tipCache != nil {
			tipHash := tipCache.BlockHash()
			if !tipHash.IsEqual(&ib.Header.PrevBlock) {
				panic("invalid canonical chain")
			}
		}

		bs.UnconfirmedBlockCache.Add(ib)
	}

	bs.logger.Infof("bootstrapping is finished at the tip confirmed height: %d",
		bs.confirmedTipBlock.Height)
}

func (bs *BtcScanner) SetLogger(logger *zap.SugaredLogger) {
	bs.logger = logger
}

func (bs *BtcScanner) GetHeadersChan() chan *wire.BlockHeader {
	return bs.blockHeaderChan
}

func (bs *BtcScanner) sendConfirmedBlocksToChan(blocks []*types.IndexedBlock) {
	for i := 0; i < len(blocks); i++ {
		bs.ConfirmedBlocksChan <- blocks[i]
		bs.confirmedTipBlock = blocks[i]
	}
}

func (bs *BtcScanner) tryToExtractCheckpoint(block *types.IndexedBlock) *types.CheckpointRecord {
	found := bs.tryToExtractCkptSegment(block)
	if !found {
		return nil
	}

	rawCheckpointWithBtcHeight, err := bs.matchAndPop()
	if err != nil {
		// if a raw checkpoint is found, it should be decoded. Otherwise
		// this means there are bugs in the program, should panic here
		panic(err)
	}

	return rawCheckpointWithBtcHeight
}

func (bs *BtcScanner) matchAndPop() (*types.CheckpointRecord, error) {
	bs.ckptCache.Match()
	ckptSegments := bs.ckptCache.PopEarliestCheckpoint()
	if ckptSegments == nil {
		return nil, nil
	}
	connectedBytes, err := btctxformatter.ConnectParts(bs.ckptCache.Version, ckptSegments.Segments[0].Data, ckptSegments.Segments[1].Data)
	if err != nil {
		return nil, fmt.Errorf("failed to connect two checkpoint parts: %w", err)
	}
	// found a pair, check if it is a valid checkpoint
	rawCheckpoint, err := ckpttypes.FromBTCCkptBytesToRawCkpt(connectedBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to decode raw checkpoint bytes: %w", err)
	}

	return &types.CheckpointRecord{
		RawCheckpoint:      rawCheckpoint,
		FirstSeenBtcHeight: uint64(ckptSegments.Segments[0].AssocBlock.Height),
	}, nil
}

func (bs *BtcScanner) tryToExtractCkptSegment(b *types.IndexedBlock) bool {
	found := false
	for _, tx := range b.Txs {
		if tx == nil {
			continue
		}

		// cache the segment to ckptCache
		ckptSeg := types.NewCkptSegment(bs.ckptCache.Tag, bs.ckptCache.Version, b, tx)
		if ckptSeg != nil {
			err := bs.ckptCache.AddSegment(ckptSeg)
			if err != nil {
				bs.logger.Errorf("Failed to add the ckpt segment in tx %v to the ckptCache: %v", tx.Hash(), err)
				continue
			}
			found = true
		}
	}
	return found
}

func (bs *BtcScanner) GetCheckpointsChan() chan *types.CheckpointRecord {
	return bs.checkpointsChan
}

func (bs *BtcScanner) Stop() {
	close(bs.quit)
}
