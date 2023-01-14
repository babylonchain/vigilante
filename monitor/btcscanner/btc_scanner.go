package btcscanner

import (
	"fmt"
	"github.com/babylonchain/babylon/btctxformatter"
	"github.com/babylonchain/vigilante/netparams"
	"github.com/btcsuite/btcd/wire"
	"go.uber.org/atomic"
	"sync"

	ckpttypes "github.com/babylonchain/babylon/x/checkpointing/types"
	"github.com/babylonchain/vigilante/btcclient"
	"github.com/babylonchain/vigilante/config"
	"github.com/babylonchain/vigilante/types"
	"github.com/btcsuite/btcutil"
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
	checkpointsChan chan *types.CheckpointBTC

	Synced *atomic.Bool

	wg      sync.WaitGroup
	started bool
	quit    chan struct{}
	quitMu  sync.Mutex
}

func New(btcCfg *config.BTCConfig, monitorCfg *config.MonitorConfig, btcClient btcclient.BTCClient, btclightclientBaseHeight uint64, btcConfirmationDepth uint64, tagID uint8) (*BtcScanner, error) {
	bbnParam := netparams.GetBabylonParams(btcCfg.NetParams, tagID)
	headersChan := make(chan *wire.BlockHeader, monitorCfg.BtcBlockBufferSize)
	confirmedBlocksChan := make(chan *types.IndexedBlock, monitorCfg.BtcBlockBufferSize)
	ckptsChan := make(chan *types.CheckpointBTC, monitorCfg.CheckpointBufferSize)
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
	}, nil
}

// Start starts the scanning process from curBTCHeight to tipHeight
func (bs *BtcScanner) Start() {
	bs.BtcClient.MustSubscribeBlocks()
	go bs.Bootstrap()
	for {
		block := bs.getNextConfirmedBlock()
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
		// move to the next BTC block
	}
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

// getNextConfirmedBlock returns the next confirmed block from the channel
func (bs *BtcScanner) getNextConfirmedBlock() *types.IndexedBlock {
	return <-bs.ConfirmedBlocksChan
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

func (bs *BtcScanner) tryToExtractCheckpoint(block *types.IndexedBlock) *types.CheckpointBTC {
	found := bs.tryToExtractCkptSegment(block.Txs)
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

func (bs *BtcScanner) matchAndPop() (*types.CheckpointBTC, error) {
	bs.ckptCache.Match()
	ckptSegments := bs.ckptCache.PopEarliestCheckpoint()
	connectedBytes, err := btctxformatter.ConnectParts(bs.ckptCache.Version, ckptSegments.Segments[0].Data, ckptSegments.Segments[1].Data)
	if err != nil {
		return nil, fmt.Errorf("failed to connect two checkpoint parts: %w", err)
	}
	// found a pair, check if it is a valid checkpoint
	rawCheckpoint, err := ckpttypes.FromBTCCkptBytesToRawCkpt(connectedBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to decode raw checkpoint bytes: %w", err)
	}

	return &types.CheckpointBTC{
		RawCheckpoint: rawCheckpoint,
		BtcHeight:     uint64(ckptSegments.Segments[0].AssocBlock.Height),
	}, nil
}

func (bs *BtcScanner) tryToExtractCkptSegment(txs []*btcutil.Tx) bool {
	found := false
	for _, tx := range txs {
		if tx == nil {
			continue
		}

		// cache the segment to ckptCache
		ckptSeg := types.NewCkptSegment(bs.ckptCache.Tag, bs.ckptCache.Version, nil, tx)
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

func (bs *BtcScanner) GetCheckpointsChan() chan *types.CheckpointBTC {
	return bs.checkpointsChan
}

// quitChan atomically reads the quit channel.
func (bs *BtcScanner) quitChan() <-chan struct{} {
	bs.quitMu.Lock()
	c := bs.quit
	bs.quitMu.Unlock()
	return c
}
