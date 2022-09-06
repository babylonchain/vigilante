package btcclient

import (
	"github.com/babylonchain/vigilante/types"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
)

func (c *Client) GetBlockByHash(blockHash *chainhash.Hash) (*types.IndexedBlock, error) {
	blockInfo, err := c.GetBlockVerbose(blockHash)
	if err != nil {
		return nil, err
	}

	mBlock, err := c.GetBlock(blockHash)
	if err != nil {
		return nil, err
	}

	btcTxs := types.GetWrappedTxs(mBlock)
	return types.NewIndexedBlock(int32(blockInfo.Height), &mBlock.Header, btcTxs), nil
}
