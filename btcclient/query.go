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

func (c *Client) GetChainBlocks(baseHeight uint64, tipHash *chainhash.Hash) ([]*types.IndexedBlock, error) {
	tipIb, mBlock, err := c.GetBlockByHash(tipHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get block by hash %x: %w", tipHash, err)
	}
	tipHeight := uint64(tipIb.Height)
	if tipHeight < baseHeight {
		return nil, fmt.Errorf("the tip block height %v is less than the base height %v", tipIb.Height, baseHeight)
	}

	chainBlocks := make([]*types.IndexedBlock, tipHeight-baseHeight)
	chainBlocks[len(chainBlocks)-1] = tipIb
	prevHash := &mBlock.Header.PrevBlock
	for i := tipHeight - baseHeight - 1; i >= 0; i-- {
		ib, mb, err := c.GetBlockByHash(prevHash)
		if err != nil {
			return nil, fmt.Errorf("failed to get block by hash %x: %w", prevHash, err)
		}
		chainBlocks[i] = ib
		prevHash = &mb.Header.PrevBlock
	}

	return chainBlocks, nil
}

// FindTailChainBlocks returns a chain of blocks cut by a given deep
func (c *Client) FindTailChainBlocks(deep uint64) ([]*types.IndexedBlock, error) {
	tipIb, err := c.getBestIndexedBlock()
	if err != nil {
		return nil, err
	}
	if uint64(tipIb.Height) <= deep {
		return nil, fmt.Errorf("the tip height of BTC %v should be higher than %v", tipIb.Height, deep)
	}
	startHeight := uint64(tipIb.Height) - deep
	tipHash := tipIb.BlockHash()
	return c.GetChainBlocks(startHeight, &tipHash)
}

func (c *Client) getBestIndexedBlock() (*types.IndexedBlock, error) {
	tipHash, err := c.GetBestBlockHash()
	if err != nil {
		return nil, fmt.Errorf("failed to get the best block %w", err)
	}
	tipIb, _, err := c.GetBlockByHash(tipHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get the block by hash %x: %w", tipHash, err)
	}

	return tipIb, nil
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
