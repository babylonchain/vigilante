package reporter

import (
	"github.com/babylonchain/vigilante/types"
	"github.com/btcsuite/btcd/wire"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (r *Reporter) indexedBlockHandler() {
	defer r.wg.Done()
	quit := r.quitChan()

	for {
		select {
		case ib := <-r.btcClient.IndexedBlockChan:
			r.btcCache.Add(ib)
			blockHash := ib.BlockHash()
			signer := r.babylonClient.MustGetAddr()
			log.Infof("Start handling block %v from BTC client", blockHash)

			// handler the BTC header, including
			// - wrap header into MsgInsertHeader message
			// - submit MsgInsertHeader msg to Babylon
			if err := r.submitHeader(signer, ib.Header); err != nil {
				log.Errorf("Failed to handle header %v from Bitcoin: %v", blockHash, err)
			}
			// TODO: ensure that the header is inserted into BTCLightclient, then filter txs
			// (see relevant discussion in https://github.com/babylonchain/vigilante/pull/5)

			// extract ckpt parts from txs,
			if err := r.extractAndSubmitCkpts(signer, ib); err != nil {
				log.Errorf("Failed to handle txs in header %v from Bitcoin: %v", blockHash, err)
			}
		case <-quit:
			// We have been asked to stop
			return
		}
	}
}

func (r *Reporter) submitHeader(signer sdk.AccAddress, header *wire.BlockHeader) error {
	msgInsertHeader := types.NewMsgInsertHeader(r.babylonClient.Cfg.AccountPrefix, signer, header)
	log.Debugf("signer: %v, headerHex: %v", signer, msgInsertHeader.Header.MarshalHex())
	res, err := r.babylonClient.InsertHeader(msgInsertHeader)
	if err != nil {
		return err
	}
	log.Infof("Successfully submitted MsgInsertHeader with header hash %v to Babylon with response %v", header.BlockHash(), res)
	return nil
}

func (r *Reporter) extractAndSubmitCkpts(signer sdk.AccAddress, ib *types.IndexedBlock) error {
	// for each tx, try to extract a ckpt segment from it.
	// If there is a ckpt segment, cache it to ckptPool locally
	numCkptSegs := 0
	for _, tx := range ib.Txs {
		// cache the segment to ckptPool
		ckptSeg := types.GetIndexedCkptSeg(r.ckptSegmentPool.Tag, r.ckptSegmentPool.Version, ib, tx)
		if ckptSeg != nil {
			log.Infof("Found a checkpoint segment in tx %v with index %d: %v", tx.Hash, ckptSeg.Index, ckptSeg.Data)
			if err := r.ckptSegmentPool.Add(ckptSeg); err != nil {
				log.Errorf("Failed to add the ckpt segment in tx %v to the pool: %v", tx.Hash, err)
				continue
			}
			numCkptSegs += 1
		}
	}

	if numCkptSegs == 0 {
		log.Infof("Block %v contains no checkpoint segment", ib.BlockHash())
		return nil
	}

	// get matched ckpt parts from the pool
	matchedPairs := r.ckptSegmentPool.Match()
	// for each matched pair, wrap to MsgInsertBTCSpvProof and send to Babylon
	for _, pair := range matchedPairs {
		proofs, err := types.CkptSegPairToSPVProofs(pair)
		if err != nil {
			log.Errorf("Failed to generate SPV proofs: %v", err)
			continue
		}
		msgInsertBTCSpvProof, err := types.NewMsgInsertBTCSpvProof(signer, proofs)
		if err != nil {
			log.Errorf("Failed to generate new MsgInsertBTCSpvProof: %v", err)
			continue
		}
		res, err := r.babylonClient.InsertBTCSpvProof(msgInsertBTCSpvProof)
		if err != nil {
			log.Errorf("Failed to insert new MsgInsertBTCSpvProof: %v", err)
			continue
		}
		log.Infof("Successfully submitted MsgInsertHeader with header hash %v to Babylon with response %v", ib.BlockHash(), res)
	}

	return nil
}
