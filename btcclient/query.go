package btcclient

import (
	"fmt"
	"github.com/babylonchain/vigilante/types"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

func (c *Client) GetBlockByHash(blockHash *chainhash.Hash) (*types.IndexedBlock, *wire.MsgBlock, error) {
	blockInfo, err := c.GetBlockVerbose(blockHash)
	if err != nil {
		return nil, nil, err
	}

	mBlock, err := c.GetBlock(blockHash)
	if err != nil {
		return nil, nil, err
	}

	btcTxs := types.GetWrappedTxs(mBlock)
	return types.NewIndexedBlock(int32(blockInfo.Height), &mBlock.Header, btcTxs), mBlock, nil
}

func (c *Client) GetLastBlocks(stopHeight int32) ([]*types.IndexedBlock, error) {
	// Get blocks from BTC up to specified height
	var (
		err             error
		prevBlockHash   *chainhash.Hash
		bestBlockHeight int32
		mBlock          *wire.MsgBlock
		ib              *types.IndexedBlock
		ibs             []*types.IndexedBlock
	)

	prevBlockHash, bestBlockHeight, err = c.GetBestBlock()
	if err != nil {
		return nil, err
	}

	if stopHeight > bestBlockHeight {
		return nil, fmt.Errorf("invalid stop height %d", stopHeight)
	}

	for {
		ib, mBlock, err = c.GetBlockByHash(prevBlockHash)
		if err != nil {
			return nil, err
		}

		ibs = append(ibs, ib)
		prevBlockHash = &mBlock.Header.PrevBlock
		if ib.Height == stopHeight {
			break
		}
	}

	return ibs, nil
}
