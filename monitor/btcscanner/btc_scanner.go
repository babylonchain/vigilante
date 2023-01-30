package btcscanner

import (
	"fmt"
	"github.com/babylonchain/babylon/btctxformatter"
	ckpttypes "github.com/babylonchain/babylon/x/checkpointing/types"
	"github.com/babylonchain/vigilante/btcclient"
	"github.com/babylonchain/vigilante/config"
	"github.com/babylonchain/vigilante/netparams"
	"github.com/babylonchain/vigilante/types"
	"github.com/btcsuite/btcd/wire"
	"go.uber.org/atomic"
	"sync"
)

type BtcScanner struct {
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

func New(btcCfg *config.BTCConfig, monitorCfg *config.MonitorConfig, btcClient btcclient.BTCClient, btclightclientBaseHeight uint64, btcConfirmationDepth uint64, tagID uint8) (*BtcScanner, error) {
	bbnParam := netparams.GetBabylonParams(btcCfg.NetParams, tagID)
	headersChan := make(chan *wire.BlockHeader, monitorCfg.BtcBlockBufferSize)
	confirmedBlocksChan := make(chan *types.IndexedBlock, monitorCfg.BtcBlockBufferSize)
	ckptsChan := make(chan *types.CheckpointRecord, monitorCfg.CheckpointBufferSize)
	ckptCache := types.NewCheckpointCache(bbnParam.Tag, bbnParam.Version)
	unconfirmedBlockCache, err := types.NewBTCCache(monitorCfg.BtcCacheSize)
	if err != nil {
		panic(fmt.Errorf("failed to create BTC cache for tail blocks: %w", err))
	}

	return &BtcScanner{
		BtcClient:             btcClient,
		BaseHeight:            btclightclientBaseHeight,
		K:                     btcConfirmationDepth,
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
		log.Info("the BTC scanner is already started")
		return
	}

	// the bootstrapping should not block the main thread
	go bs.Bootstrap()

	bs.BtcClient.MustSubscribeBlocks()

	bs.Started.Store(true)
	log.Info("the BTC scanner is started")

	// start handling new blocks
	bs.wg.Add(1)
	go bs.blockEventHandler()

	for bs.Started.Load() {
		select {
		case <-bs.quit:
			bs.Started.Store(false)
		case block := <-bs.ConfirmedBlocksChan:
			// send the header to the Monitor for consistency check
			bs.blockHeaderChan <- block.Header
			ckptBtc := bs.tryToExtractCheckpoint(block)
			if ckptBtc == nil {
				log.Debugf("checkpoint not found at BTC block %v", block.Height)
				// move to the next BTC block
				continue
			}
			log.Infof("got a checkpoint at BTC block %v", block.Height)

			bs.checkpointsChan <- ckptBtc
		}
	}

	bs.wg.Wait()
	log.Info("the BTC scanner is stopped")
}

// Bootstrap syncs with BTC by getting the confirmed blocks and the caching the unconfirmed blocks
func (bs *BtcScanner) Bootstrap() {
	var (
		firstUnconfirmedHeight uint64
		chainBlocks            []*types.IndexedBlock
		confirmedBlocks        []*types.IndexedBlock
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

	log.Infof("the bootstrapping starts at %d", firstUnconfirmedHeight)

	chainBlocks, err = bs.BtcClient.FindTailBlocksByHeight(firstUnconfirmedHeight)
	if err != nil {
		panic(fmt.Errorf("failed to find the tail chain with base height %d: %w", bs.BaseHeight, err))
	}

	err = bs.UnconfirmedBlockCache.Init(chainBlocks)
	if err != nil {
		panic(fmt.Errorf("failed to initialize BTC cache for tail blocks: %w", err))
	}

	confirmedBlocks = bs.UnconfirmedBlockCache.TrimConfirmedBlocks(int(bs.K))
	if confirmedBlocks == nil {
		return
	}

	// if the scanner was bootstrapped before, the new confirmed canonical chain must connect to the previous one
	if bs.confirmedTipBlock != nil {
		confirmedTipHash := bs.confirmedTipBlock.BlockHash()
		if !confirmedTipHash.IsEqual(&confirmedBlocks[0].Header.PrevBlock) {
			panic("invalid canonical chain")
		}
	}

	bs.sendConfirmedBlocksToChan(confirmedBlocks)
}

func (bs *BtcScanner) GetHeadersChan() chan *wire.BlockHeader {
	return bs.blockHeaderChan
}

func (bs *BtcScanner) sendConfirmedBlocksToChan(blocks []*types.IndexedBlock) {
	for i := 0; i < len(blocks); i++ {
		bs.ConfirmedBlocksChan <- blocks[i]
	}
	bs.confirmedTipBlock = blocks[len(blocks)-1]
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
				log.Errorf("Failed to add the ckpt segment in tx %v to the ckptCache: %v", tx.Hash(), err)
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
