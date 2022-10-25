package reporter

import (
	"github.com/babylonchain/babylon/types/retry"
	btcctypes "github.com/babylonchain/babylon/x/btccheckpoint/types"
	btclctypes "github.com/babylonchain/babylon/x/btclightclient/types"
	"github.com/babylonchain/vigilante/types"
	"github.com/btcsuite/btcd/wire"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// mustSubmitHeaders submits unique headers to Babylon and panics if it fails
func (r *Reporter) mustSubmitHeaders(signer sdk.AccAddress, headers []*wire.BlockHeader) {
	var err error

	err = retry.Do(r.retrySleepTime, r.maxRetrySleepTime, func() error {
		var (
			msgs       []*btclctypes.MsgInsertHeader
			res        *sdk.TxResponse
			startPoint = -1
			contained  bool
		)

		// find the first header that is not contained in BBN header chain, then submit since this header
		for i, header := range headers {
			blockHash := header.BlockHash()
			contained, err = r.babylonClient.QueryContainsBlock(&blockHash)
			if err != nil {
				panic(err)
			}
			if !contained {
				startPoint = i
				break
			}
		}

		// all headers are duplicated, no need to submit
		if startPoint == -1 {
			log.Info("All headers are duplicated, no need to submit")
			return nil
		}

		headersToSubmit := headers[startPoint:]
		for _, header := range headersToSubmit {
			msgInsertHeader := types.NewMsgInsertHeader(r.babylonClient.Cfg.AccountPrefix, signer, header)
			msgs = append(msgs, msgInsertHeader)
		}
		res, err = r.babylonClient.InsertHeaders(msgs)
		if err != nil {
			return err
		}

		log.Infof("Successfully submitted %d headers to Babylon with response code %v", len(msgs), res.Code)
		return nil
	})
	if err != nil {
		log.Errorf("Failed to submit headers to Babylon: %v", err)
		panic(err)
	}
}

func (r *Reporter) extractCheckpoints(ib *types.IndexedBlock) int {
	// for each tx, try to extract a ckpt segment from it.
	// If there is a ckpt segment, cache it to ckptCache locally
	numCkptSegs := 0

	for _, tx := range ib.Txs {
		if tx == nil {
			log.Warnf("Found a nil tx in block %v", ib.BlockHash())
			continue
		}

		// cache the segment to ckptCache
		ckptSeg := types.NewCkptSegment(r.CheckpointCache.Tag, r.CheckpointCache.Version, ib, tx)
		if ckptSeg != nil {
			log.Infof("Found a checkpoint segment in tx %v with index %d: %v", tx.Hash(), ckptSeg.Index, ckptSeg.Data)
			if err := r.CheckpointCache.AddSegment(ckptSeg); err != nil {
				log.Errorf("Failed to add the ckpt segment in tx %v to the ckptCache: %v", tx.Hash(), err)
				continue
			}
			numCkptSegs += 1
		}
	}

	return numCkptSegs
}

func (r *Reporter) matchAndSubmitCheckpoints(signer sdk.AccAddress) error {
	var (
		res                  *sdk.TxResponse
		proofs               []*btcctypes.BTCSpvProof
		msgInsertBTCSpvProof *btcctypes.MsgInsertBTCSpvProof
		err                  error
	)

	// get matched ckpt parts from the ckptCache
	// Note that Match() has ensured the checkpoints are always ordered by epoch number
	r.CheckpointCache.Match()

	if r.CheckpointCache.NumCheckpoints() == 0 {
		log.Debug("Found no matched pair of checkpoint segments in this match attempt")
		return nil
	}

	// for each matched checkpoint, wrap to MsgInsertBTCSpvProof and send to Babylon
	// Note that this is a while loop that keeps poping checkpoints in the cache
	for {
		// pop the earliest checkpoint
		// if poping a nil checkpoint, then all checkpoints are poped, break the for loop
		ckpt := r.CheckpointCache.PopEarliestCheckpoint()
		if ckpt == nil {
			break
		}

		log.Info("Found a matched pair of checkpoint segments!")

		// fetch the first checkpoint in cache and construct spv proof
		proofs, err = ckpt.GenSPVProofs()
		if err != nil {
			log.Errorf("Failed to generate SPV proofs: %v", err)
			continue
		}

		// report this checkpoint to Babylon
		msgInsertBTCSpvProof, err = types.NewMsgInsertBTCSpvProof(signer, proofs)
		if err != nil {
			log.Errorf("Failed to generate new MsgInsertBTCSpvProof: %v", err)
			continue
		}
		res = r.babylonClient.MustInsertBTCSpvProof(msgInsertBTCSpvProof)
		log.Infof("Successfully submitted MsgInsertBTCSpvProof with response %d", res.Code)
	}

	return nil
}

func (r *Reporter) processBlocks(signer sdk.AccAddress, ibs []*types.IndexedBlock) {
	var (
		headers     []*wire.BlockHeader
		numCkptSegs int
	)

	// extract headers and ckpt segments from the blocks
	for _, ib := range ibs {
		headers = append(headers, ib.Header)
		numCkptSegs += r.extractCheckpoints(ib)
	}

	// submit headers to Babylon
	r.mustSubmitHeaders(signer, headers)

	if numCkptSegs > 0 {
		log.Infof("Found %d checkpoint segments", numCkptSegs)
	}

	// match and submit checkpoint segments
	if err := r.matchAndSubmitCheckpoints(signer); err != nil {
		log.Errorf("Failed to match and submit ckpts: %v", err)
	}
}
