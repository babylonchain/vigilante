package reporter

import (
	"fmt"

	"github.com/babylonchain/babylon/types/retry"
	btcctypes "github.com/babylonchain/babylon/x/btccheckpoint/types"
	btclctypes "github.com/babylonchain/babylon/x/btclightclient/types"
	"github.com/btcsuite/btcd/wire"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonchain/vigilante/types"
)

// submitHeadersDedup submits unique headers to Babylon.
// It returns the number of headers that it submits after deduplication
func (r *Reporter) submitHeadersDedup(signer sdk.AccAddress, headers []*wire.BlockHeader) (int, error) {
	var (
		tempHeaders  = headers
		numSubmitted = 0
		err          error
	)

	err = retry.Do(r.retrySleepTime, r.maxRetrySleepTime, func() error {
		var (
			msgs []*btclctypes.MsgInsertHeader
			res  *sdk.TxResponse
		)

		headersToSubmit := r.findHeadersToSubmit(tempHeaders)
		if len(headersToSubmit) == 0 {
			log.Info("No new headers to submit")
			return nil
		}

		tempHeaders = headersToSubmit
		numSubmitted = len(headersToSubmit)
		for _, header := range headersToSubmit {
			msgInsertHeader := types.NewMsgInsertHeader(r.babylonClient.GetConfig().AccountPrefix, signer, header)
			msgs = append(msgs, msgInsertHeader)
		}
		// TODO would this cause any issues if the number of unsubmitted headers is very large?
		res, err = r.babylonClient.InsertHeaders(msgs)
		if err != nil {
			return err
		}

		log.Infof("Successfully submitted %d headers to Babylon with response code %v", len(msgs), res.Code)
		return nil
	})

	if err != nil {
		r.metrics.FailedHeadersCounter.Add(float64(numSubmitted))
		return 0, fmt.Errorf("failed to submit headers: %w", err)
	}

	r.metrics.SuccessfulHeadersCounter.Add(float64(numSubmitted))
	r.metrics.SecondsSinceLastHeaderGauge.Set(0)

	return numSubmitted, err
}

func (r *Reporter) findHeadersToSubmit(headers []*wire.BlockHeader) []*wire.BlockHeader {
	var (
		startPoint      = -1
		headersToSubmit []*wire.BlockHeader
	)

	// find the first header that is not contained in BBN header chain, then submit since this header
	for i, header := range headers {
		blockHash := header.BlockHash()
		res, err := r.babylonClient.ContainsBTCBlock(&blockHash)
		if err != nil {
			panic(err)
		}
		if !res.Contains {
			startPoint = i
			break
		}
	}

	// all headers are duplicated, no need to submit
	if startPoint == -1 {
		log.Info("All headers are duplicated, no need to submit")
		return headersToSubmit
	}

	headersToSubmit = headers[startPoint:]
	return headersToSubmit
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

func (r *Reporter) matchAndSubmitCheckpoints(signer sdk.AccAddress) (int, error) {
	var (
		res                  *sdk.TxResponse
		proofs               []*btcctypes.BTCSpvProof
		msgInsertBTCSpvProof *btcctypes.MsgInsertBTCSpvProof
		err                  error
	)

	// get matched ckpt parts from the ckptCache
	// Note that Match() has ensured the checkpoints are always ordered by epoch number
	r.CheckpointCache.Match()
	numMatchedCkpts := r.CheckpointCache.NumCheckpoints()

	if numMatchedCkpts == 0 {
		log.Debug("Found no matched pair of checkpoint segments in this match attempt")
		return numMatchedCkpts, nil
	}

	// for each matched checkpoint, wrap to MsgInsertBTCSpvProof and send to Babylon
	// Note that this is a while loop that keeps popping checkpoints in the cache
	for {
		// pop the earliest checkpoint
		// if popping a nil checkpoint, then all checkpoints are popped, break the for loop
		ckpt := r.CheckpointCache.PopEarliestCheckpoint()
		if ckpt == nil {
			break
		}

		log.Info("Found a matched pair of checkpoint segments!")

		// fetch the first checkpoint in cache and construct spv proof
		proofs = ckpt.MustGenSPVProofs()

		// wrap to MsgInsertBTCSpvProof
		msgInsertBTCSpvProof = types.MustNewMsgInsertBTCSpvProof(signer, proofs)

		// submit the checkpoint to Babylon
		res, err = r.babylonClient.InsertBTCSpvProof(msgInsertBTCSpvProof)
		if err != nil {
			log.Errorf("Failed to submit MsgInsertBTCSpvProof with error %v", err)
			r.metrics.FailedCheckpointsCounter.Inc()
			continue
		}
		log.Infof("Successfully submitted MsgInsertBTCSpvProof with response %d", res.Code)
		r.metrics.SuccessfulCheckpointsCounter.Inc()
		r.metrics.SecondsSinceLastCheckpointGauge.Set(0)
	}

	return numMatchedCkpts, nil
}

// ProcessCheckpoints tries to extract checkpoint segments from a list of blocks, find matched checkpoint segments, and report matched checkpoints
// It returns the number of extracted checkpoint segments, and the number of matched checkpoints
func (r *Reporter) ProcessCheckpoints(signer sdk.AccAddress, ibs []*types.IndexedBlock) (int, int, error) {
	var numCkptSegs int

	// extract ckpt segments from the blocks
	for _, ib := range ibs {
		numCkptSegs += r.extractCheckpoints(ib)
	}

	if numCkptSegs > 0 {
		log.Infof("Found %d checkpoint segments", numCkptSegs)
	}

	// match and submit checkpoint segments
	numMatchedCkpts, err := r.matchAndSubmitCheckpoints(signer)

	return numCkptSegs, numMatchedCkpts, err
}

// ProcessHeaders extracts and reports headers from a list of blocks
// It returns the number of headers that need to be reported (after deduplication)
func (r *Reporter) ProcessHeaders(signer sdk.AccAddress, ibs []*types.IndexedBlock) (int, error) {
	var (
		headers []*wire.BlockHeader
	)

	// extract headers from ibs
	for _, ib := range ibs {
		headers = append(headers, ib.Header)
	}

	// submit headers to Babylon
	return r.submitHeadersDedup(signer, headers)
}
