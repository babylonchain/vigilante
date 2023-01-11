package btcscanner

import (
	"errors"
	"fmt"
	"github.com/babylonchain/babylon/btctxformatter"
	"github.com/babylonchain/vigilante/netparams"

	ckpttypes "github.com/babylonchain/babylon/x/checkpointing/types"
	"github.com/babylonchain/vigilante/btcclient"
	"github.com/babylonchain/vigilante/config"
	"github.com/babylonchain/vigilante/log"
	"github.com/babylonchain/vigilante/types"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
)

type BtcScanner struct {
	// connect to BTC node
	btcClient *btcclient.Client
	// cache a sequence of checkpoints
	ckptCache *types.CheckpointCache

	curBTCHeight uint64
	tipHeight    uint64

	// communicate with the monitor
	verificationChan chan *ckpttypes.RawCheckpoint
}

func New(cfg *config.BTCConfig, btcClient *btcclient.Client, btclightclientBaseHeight uint64, tagID uint8, verificationBuffer uint64) (*BtcScanner, error) {
	tipBlock, err := btcClient.GetTipBlockVerbose()
	if err != nil {
		return nil, err
	}
	tipHeight := uint64(tipBlock.Height)
	if tipHeight < btclightclientBaseHeight {
		return nil, errors.New(fmt.Sprintf("invalid BTC base height %v, tip height is %v", btclightclientBaseHeight, tipHeight))
	}

	bbnParam := netparams.GetBabylonParams(cfg.NetParams, tagID)
	verificationChan := make(chan *ckpttypes.RawCheckpoint, verificationBuffer)
	ckptCache := types.NewCheckpointCache(bbnParam.Tag, bbnParam.Version)

	return &BtcScanner{
		btcClient:        btcClient,
		curBTCHeight:     btclightclientBaseHeight,
		tipHeight:        tipHeight,
		ckptCache:        ckptCache,
		verificationChan: verificationChan,
	}, nil
}

// Start starts the scanning process from curBTCHeight to tipHeight
func (bs *BtcScanner) Start() {
	for bs.curBTCHeight <= bs.tipHeight {
		block, err := bs.getBlockAtCurrentHeight()
		if err != nil {
			panic(fmt.Errorf("cannot get BTC block from the BTC node: %w", err))
		}
		ckpt := bs.tryToExtractCheckpoint(block)
		if ckpt == nil {
			log.Logger.Debugf("checkpoint not found at BTC block %v", bs.curBTCHeight)
			// move to the next BTC block
			bs.curBTCHeight++
			continue
		}
		log.Logger.Infof("got a checkpoint at BTC block %v", bs.curBTCHeight)

		bs.verificationChan <- ckpt
		// move to the next BTC block
		bs.curBTCHeight++
	}
}

func (bs *BtcScanner) getBlockAtCurrentHeight() (*wire.MsgBlock, error) {
	bh, err := bs.btcClient.GetBlockHash(int64(bs.curBTCHeight))
	if err != nil {
		return nil, err
	}
	block, err := bs.btcClient.GetBlock(bh)
	if err != nil {
		return nil, err
	}

	return block, nil
}

func (bs *BtcScanner) tryToExtractCheckpoint(block *wire.MsgBlock) *ckpttypes.RawCheckpoint {
	txs := types.GetWrappedTxs(block)
	found := bs.tryToExtractCkptSegment(txs)
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
				log.Logger.Errorf("Failed to add the ckpt segment in tx %v to the ckptCache: %v", tx.Hash(), err)
				continue
			}
			found = true
		}
	}
	return found
}

func (bs *BtcScanner) GetNextCheckpoint() *ckpttypes.RawCheckpoint {
	return <-bs.verificationChan
}
