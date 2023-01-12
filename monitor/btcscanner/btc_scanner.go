package btcscanner

import (
	"fmt"
	"github.com/babylonchain/babylon/btctxformatter"
	"github.com/babylonchain/vigilante/netparams"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
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

	lastCanonicalBlockHash *chainhash.Hash
	canonicalTipHeight     uint64
	CanonicalBlocksChan    chan *types.IndexedBlock

	// cache a sequence of checkpoints
	ckptCache *types.CheckpointCache
	// cache unconfirmed blocks
	UnconfirmedBlocks *types.BTCCache

	// communicate with the monitor
	blockHeaderChan chan *wire.BlockHeader
	checkpointsChan chan *ckpttypes.RawCheckpoint

	Synced *atomic.Bool

	wg      sync.WaitGroup
	started bool
	quit    chan struct{}
	quitMu  sync.Mutex
}

func New(cfg *config.BTCConfig, btcClient btcclient.BTCClient, btclightclientBaseHeight uint64, btcConfirmationDepth uint64, tagID uint8, blockBuffer uint64, checkpointBuffer uint64) (*BtcScanner, error) {
	bbnParam := netparams.GetBabylonParams(cfg.NetParams, tagID)
	headersChan := make(chan *wire.BlockHeader, blockBuffer)
	canonicalBlocksChan := make(chan *types.IndexedBlock, blockBuffer)
	ckptsChan := make(chan *ckpttypes.RawCheckpoint, checkpointBuffer)
	ckptCache := types.NewCheckpointCache(bbnParam.Tag, bbnParam.Version)
	tailBlocks, err := types.NewBTCCache(btcConfirmationDepth)
	if err != nil {
		panic(fmt.Errorf("failed to create BTC cache for tail blocks: %w", err))
	}

	return &BtcScanner{
		BtcClient:           btcClient,
		BaseHeight:          btclightclientBaseHeight,
		K:                   btcConfirmationDepth,
		ckptCache:           ckptCache,
		UnconfirmedBlocks:   tailBlocks,
		CanonicalBlocksChan: canonicalBlocksChan,
		blockHeaderChan:     headersChan,
		checkpointsChan:     ckptsChan,
		Synced:              atomic.NewBool(false),
	}, nil
}

// Start starts the scanning process from curBTCHeight to tipHeight
func (bs *BtcScanner) Start() {
	bs.BtcClient.MustSubscribeBlocks()
	go bs.Bootstrap()
	for {
		block := bs.GetNextCanonicalBlock()
		// TODO check header consistency with Babylon
		ckpt := bs.tryToExtractCheckpoint(block)
		if ckpt == nil {
			log.Debugf("checkpoint not found at BTC block %v", block.Height)
			// move to the next BTC block
			continue
		}
		log.Infof("got a checkpoint at BTC block %v", block.Height)

		bs.checkpointsChan <- ckpt
		// move to the next BTC block
	}
}

// Bootstrap syncs with BTC by getting the canonical chain and the caching the unconfirmed chain
func (bs *BtcScanner) Bootstrap() {
	var (
		baseHeight       uint64
		chainBlocks      []*types.IndexedBlock
		canonicalChain   []*types.IndexedBlock
		unconfirmedChain []*types.IndexedBlock
		err              error
	)

	if bs.Synced.Load() {
		// the scanner is already synced
		return
	}

	log.Infof("the bootstrapping starts at %d", bs.canonicalTipHeight)

	if bs.canonicalTipHeight == 0 {
		// first time bootstrapping
		baseHeight = bs.BaseHeight
	} else {
		baseHeight = bs.canonicalTipHeight + 1
	}
	chainBlocks, err = bs.BtcClient.FindTailBlocksByHeight(baseHeight)
	if err != nil {
		panic(fmt.Errorf("failed to find the tail chain with base height %d: %w", bs.BaseHeight, err))
	}

	// no confirmed blocks
	if len(chainBlocks) <= int(bs.K) {
		err = bs.UnconfirmedBlocks.Init(chainBlocks)
		if err != nil {
			panic(fmt.Errorf("failed to initialize BTC cache for tail blocks: %w", err))
		}
		return
	}

	// if the scanner was bootstrapped before, the new canonical chain must connect to the previous one
	if bs.lastCanonicalBlockHash != nil && *bs.lastCanonicalBlockHash != chainBlocks[0].Header.PrevBlock {
		panic("invalid canonical chain")
	}

	// split the chain of blocks into canonical chain and unconfirmed chain
	canonicalChain = chainBlocks[:len(chainBlocks)-int(bs.K)-1]
	unconfirmedChain = chainBlocks[len(chainBlocks)-int(bs.K):]
	err = bs.UnconfirmedBlocks.Init(unconfirmedChain)
	if err != nil {
		panic(fmt.Errorf("failed to initialize BTC cache for tail blocks: %w", err))
	}

	bs.sendCanonicalBlocksToChan(canonicalChain)

	bs.Synced.Store(true)
}

// GetNextCanonicalBlock returns the next canonical block from the channel
func (bs *BtcScanner) GetNextCanonicalBlock() *types.IndexedBlock {
	return <-bs.CanonicalBlocksChan
}

func (bs *BtcScanner) sendCanonicalBlocksToChan(blocks []*types.IndexedBlock) {
	for i := 0; i < len(blocks); i++ {
		bs.CanonicalBlocksChan <- blocks[i]
	}
	tipHash := blocks[len(blocks)-1].BlockHash()
	bs.lastCanonicalBlockHash = &tipHash
	bs.canonicalTipHeight = uint64(blocks[len(blocks)-1].Height)
}

func (bs *BtcScanner) tryToExtractCheckpoint(block *types.IndexedBlock) *ckpttypes.RawCheckpoint {
	found := bs.tryToExtractCkptSegment(block.Txs)
	if !found {
		return nil
	}

	rawCheckpoint, err := bs.matchAndPop()
	if err != nil {
		// if a raw checkpoint is found, it should be decoded. Otherwise
		// this means there are bugs in the program, should panic here
		panic(err)
	}

	return rawCheckpoint
}

func (bs *BtcScanner) matchAndPop() (*ckpttypes.RawCheckpoint, error) {
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

	return rawCheckpoint, nil
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

func (bs *BtcScanner) GetNextCheckpoint() *ckpttypes.RawCheckpoint {
	return <-bs.checkpointsChan
}

// quitChan atomically reads the quit channel.
func (bs *BtcScanner) quitChan() <-chan struct{} {
	bs.quitMu.Lock()
	c := bs.quit
	bs.quitMu.Unlock()
	return c
}
