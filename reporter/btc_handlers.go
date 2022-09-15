package reporter

import (
	btcctypes "github.com/babylonchain/babylon/x/btccheckpoint/types"
	"github.com/babylonchain/vigilante/types"
	"github.com/btcsuite/btcd/wire"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"strings"
)

func (r *Reporter) indexedBlockHandler() {
	defer r.wg.Done()
	quit := r.quitChan()

	signer := r.babylonClient.MustGetAddr()
	for {
		select {
		case ib := <-r.btcClient.IndexedBlockChan:
			r.btcCache.Add(ib)
			blockHash := ib.BlockHash()
			log.Infof("Start handling block %v from BTC client", blockHash)

			// handler the BTC header, including
			// - wrap header into MsgInsertHeader message
			// - submit MsgInsertHeader msg to Babylon
			if err := r.submitHeader(signer, ib.Header); err != nil {
				log.Errorf("Failed to handle header %v from Bitcoin: %v", blockHash, err)
				panic(err)
			}

			// TODO: ensure that the header is inserted into BTCLightclient, then filter txs
			// (see relevant discussion in https://github.com/babylonchain/vigilante/pull/5)

			// extract ckpt parts from txs, find matched ckpts, and submit
			numCkptSegs := r.extractCkpts(ib)
			if numCkptSegs == 0 {
				log.Infof("Block %v contains no checkpoint segment", ib.BlockHash())
			} else {
				if err := r.matchAndSubmitCkpts(signer); err != nil {
					log.Errorf("Failed to match and submit checkpoints to BBN: %v", err)
				}
			}
		case <-quit:
			// We have been asked to stop
			return
		}
	}
}

func (r *Reporter) submitHeader(signer sdk.AccAddress, header *wire.BlockHeader) error {
	err := types.Retry(3, func() error {
		msgInsertHeader := types.NewMsgInsertHeader(r.babylonClient.Cfg.AccountPrefix, signer, header)
		res, err := r.babylonClient.InsertHeader(msgInsertHeader)
		if err != nil {
			// Ignore error if header is duplicate
			if strings.Contains(err.Error(), "duplicate header") {
				log.Errorf("Ignoring error %v", err)
				return nil
			}
			return err
		}

		log.Infof("Successfully submitted MsgInsertHeader with header hash %v to Babylon with response code %v", header.BlockHash(), res.Code)
		return nil
	})

	return err
}

func (r *Reporter) extractCkpts(ib *types.IndexedBlock) int {
	// for each tx, try to extract a ckpt segment from it.
	// If there is a ckpt segment, cache it to ckptPool locally
	numCkptSegs := 0

	for _, tx := range ib.Txs {
		if tx == nil { // TODO: find out why tx can be nil
			log.Warnf("Found a nil tx in block %v", ib.BlockHash())
			continue
		}

		// cache the segment to ckptPool
		ckptSeg := types.GetIndexedCkptSeg(r.ckptSegmentPool.Tag, r.ckptSegmentPool.Version, ib, tx)
		if ckptSeg != nil {
			log.Infof("Found a checkpoint segment in tx %v with index %d: %v", tx.Hash(), ckptSeg.Index, ckptSeg.Data)
			if err := r.ckptSegmentPool.Add(ckptSeg); err != nil {
				log.Errorf("Failed to add the ckpt segment in tx %v to the pool: %v", tx.Hash(), err)
				continue
			}
			numCkptSegs += 1
		}
	}
	return numCkptSegs
}

func (r *Reporter) matchAndSubmitCkpts(signer sdk.AccAddress) error {
	var (
		res                  *sdk.TxResponse
		proofs               []*btcctypes.BTCSpvProof
		msgInsertBTCSpvProof *btcctypes.MsgInsertBTCSpvProof
		matchedPairs         [][]*types.CkptSegment
		err                  error
	)

	// get matched ckpt parts from the pool
	matchedPairs = r.ckptSegmentPool.Match()

	// for each matched pair, wrap to MsgInsertBTCSpvProof and send to Babylon
	for _, pair := range matchedPairs {
		proofs, err = types.CkptSegPairToSPVProofs(pair)
		if err != nil {
			log.Errorf("Failed to generate SPV proofs: %v", err)
			continue
		}

		msgInsertBTCSpvProof, err = types.NewMsgInsertBTCSpvProof(signer, proofs)
		if err != nil {
			log.Errorf("Failed to generate new MsgInsertBTCSpvProof: %v", err)
			continue
		}

		err = types.Retry(3, func() error {
			res, err = r.babylonClient.InsertBTCSpvProof(msgInsertBTCSpvProof)
			return err
		})
		if err != nil {
			log.Errorf("Failed to insert new MsgInsertBTCSpvProof: %v", err)
			continue
		}

		log.Infof("Successfully submitted MsgInsertBTCSpvProof with response %v", res)
	}

	return nil
}
