package btcclient

import (
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

func (c *Client) GetBlocks(stopHeight uint64) ([]*types.IndexedBlock, error) {
	var (
		err           error
		prevBlockHash *chainhash.Hash
		mBlock        *wire.MsgBlock
		ib            *types.IndexedBlock
		ibs           []*types.IndexedBlock
	)

	prevBlockHash, _, err = c.GetBestBlock()
	if err != nil {
		return nil, err
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

	return ibs, nil
}
