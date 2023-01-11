package btcscanner

import (
	"fmt"
	"github.com/babylonchain/babylon/btctxformatter"
	"github.com/babylonchain/vigilante/netparams"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
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
	CanonicalBlocksChan    chan *types.IndexedBlock

	// cache a sequence of checkpoints
	ckptCache *types.CheckpointCache
	// cache tail blocks
	TailBlocks *types.BTCCache

	// communicate with the monitor
	blockHeaderChan chan *wire.BlockHeader
	checkpointsChan chan *ckpttypes.RawCheckpoint

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
		TailBlocks:          tailBlocks,
		CanonicalBlocksChan: canonicalBlocksChan,
		blockHeaderChan:     headersChan,
		checkpointsChan:     ckptsChan,
	}, nil
}

// Start starts the scanning process from curBTCHeight to tipHeight
func (bs *BtcScanner) Start() {
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

// Bootstrap gets the canonical chain and the caches tail chain
func (bs *BtcScanner) Bootstrap() {
	tailChain, err := bs.BtcClient.FindTailChainBlocks(bs.K)
	if err != nil {
		panic(fmt.Errorf("failed to find the tail chain with %v deep: %w", bs.K, err))
	}
	err = bs.TailBlocks.Init(tailChain)

	bs.lastCanonicalBlockHash = &tailChain[0].Header.PrevBlock
	tipCanonicalBlock, _, err := bs.BtcClient.GetBlockByHash(bs.lastCanonicalBlockHash)
	if err != nil {
		panic(fmt.Errorf("failed to find the block by hash %x: %w", bs.lastCanonicalBlockHash, err))
	}
	canonicalChain, err := bs.BtcClient.GetChainBlocks(bs.BaseHeight, tipCanonicalBlock)
	if err != nil {
		panic(fmt.Errorf("failed to get the canonical chain with tip hash %x: %w", bs.lastCanonicalBlockHash, err))
	}

	bs.BtcClient.MustSubscribeBlocks()
	for i := 0; i < len(canonicalChain); i++ {
		bs.CanonicalBlocksChan <- canonicalChain[i]
	}
}

// GetNextCanonicalBlock returns the next canonical block from the channel
func (bs *BtcScanner) GetNextCanonicalBlock() *types.IndexedBlock {
	return <-bs.CanonicalBlocksChan
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
