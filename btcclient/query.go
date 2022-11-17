package btcclient

import (
	"fmt"

	"github.com/babylonchain/vigilante/types"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

// GetBestBlock provides similar functionality with the btcd.rpcclient.GetBestBlock function
// We implement this, because this function is only provided by btcd.
func (c *Client) GetBestBlock() (*chainhash.Hash, uint64, error) {
	btcLatestBlockHash, err := c.GetBestBlockHash()
	if err != nil {
		return nil, 0, err
	}
	btcLatestBlock, err := c.GetBlockVerbose(btcLatestBlockHash)
	if err != nil {
		return nil, 0, err
	}
	btcLatestBlockHeight := uint64(btcLatestBlock.Height)
	return btcLatestBlockHash, btcLatestBlockHeight, nil
}

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

// GetLastBlocks returns the last blocks from BTC up to the given height sorted in ascending order by height.
func (c *Client) GetLastBlocks(stopHeight uint64) ([]*types.IndexedBlock, error) {
	var (
		err             error
		prevBlockHash   *chainhash.Hash
		bestBlockHeight uint64
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
		if uint64(ib.Height) == stopHeight {
			break
		}
	}

	// reverse the blocks to ensure order is ascending by height
	for i, j := 0, len(ibs)-1; i < j; i, j = i+1, j-1 {
		ibs[i], ibs[j] = ibs[j], ibs[i]
	}

	return ibs, nil
}
