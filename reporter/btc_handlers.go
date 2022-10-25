package reporter

import (
	"errors"

	"github.com/babylonchain/babylon/types/retry"
	btcctypes "github.com/babylonchain/babylon/x/btccheckpoint/types"
	btclctypes "github.com/babylonchain/babylon/x/btclightclient/types"
	"github.com/babylonchain/vigilante/types"
	"github.com/btcsuite/btcd/wire"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (r *Reporter) blockEventHandler() {
	defer r.wg.Done()
	quit := r.quitChan()

	signer := r.babylonClient.MustGetAddr()
	for {
		select {
		case event := <-r.btcClient.BlockEventChan:
			if event.EventType == types.BlockConnected {
				// get the block from hash
				blockHash := event.Header.BlockHash()
				ib, mBlock, err := r.btcClient.GetBlockByHash(&blockHash)
				if err != nil {
					log.Errorf("Failed to get block %v from BTC client: %v", blockHash, err)
					panic(err)
				}

				// get cache tip
				cacheTip, err := r.btcCache.Tip()
				if err != nil {
					if errors.Is(err, types.ErrEmptyCache) {
						log.Errorf("Cache is empty, restart bootstrap process")
						r.Init(true)
						return
					}

					log.Errorf("Failed to get cache tip: %v", err)
					panic(err)
				}

				parentHash := mBlock.Header.PrevBlock

				// if the parent of the block is not the tip of the cache, then the cache is not up-to-date,
				// and we might have missed some blocks. In this case, restart the bootstrap process.
				if parentHash != cacheTip.BlockHash() {
					r.Init(true)
				} else {
					// otherwise, add the block to the cache
					if err := r.btcCache.Add(ib); err != nil {
						log.Errorf("Failed to add block %v to cache: %v", blockHash, err)
						panic(err)
					}

					log.Infof("Start handling block %v with %d txs at height %d from BTC client", blockHash, len(ib.Txs), ib.Height)

					// handle block header
					// wrap the block header in a MsgInsertHeader
					// submit the MsgInsertHeader to Babylon
					if err = r.submitHeader(signer, ib.Header); err != nil {
						log.Errorf("Failed to handle header %v from Bitcoin: %v", blockHash, err)
						panic(err)
					}

					// extract checkpoint from the block
					numCkptSegs := r.extractCkpts(ib)
					log.Infof("Block %v contains %d checkpoint segment", ib.BlockHash(), numCkptSegs)

					// if there is a checkpoint segment in the block, submit it to Babylon
					if numCkptSegs > 0 {
						if err := r.matchAndSubmitCkpts(signer); err != nil {
							log.Errorf("Failed to match and submit checkpoints to BBN: %v", err)
						}
					}
				}
			} else if event.EventType == types.BlockDisconnected {
				// get cache tip
				cacheTip, err := r.btcCache.Tip()
				if err != nil {
					if errors.Is(err, types.ErrEmptyCache) {
						log.Errorf("Cache is empty, restart bootstrap process")
						r.Init(true)
						return
					}

					log.Errorf("Failed to get cache tip: %v", err)
					panic(err)
				}

				// if the block to be disconnected is not the tip of the cache, then the cache is not up-to-date,
				if event.Header.BlockHash() != cacheTip.BlockHash() {
					r.Init(true)
				} else {
					// otherwise, remove the block from the cache
					if err := r.btcCache.RemoveLast(); err != nil {
						log.Errorf("Failed to remove last block from cache: %v", err)
						panic(err)
					}
				}

				// TODO: upon a block is disconnected,
				// - for each ckpt segment in the block:
				//   - if the segment has a matched segment:
				//     - remove the checkpoint in the checkpoint list
				//     - add the matched segment back to the segment map
				//   - else:
				//     - remove the segment from the segment map
			}

		case <-quit:
			// We have been asked to stop
			return
		}
	}
}

func (r *Reporter) submitHeader(signer sdk.AccAddress, header *wire.BlockHeader) error {
	var (
		res *sdk.TxResponse
		err error
	)

	err = retry.Do(r.retrySleepTime, r.maxRetrySleepTime, func() error {
		//TODO implement retry mechanism in mustSubmitHeader and keep submitHeader as it is
		msgInsertHeader := types.NewMsgInsertHeader(r.babylonClient.Cfg.AccountPrefix, signer, header)
		res, err = r.babylonClient.InsertHeader(msgInsertHeader)
		if err != nil {
			return err
		}

		log.Infof("Successfully submitted MsgInsertHeader with header hash %v to Babylon with response code %v", header.BlockHash(), res.Code)
		return nil
	})

	return err
}

func (r *Reporter) submitHeaders(signer sdk.AccAddress, headers []*wire.BlockHeader) error {
	var (
		contained  bool
		startPoint int
		res        *sdk.TxResponse
		err        error
	)

	// find the first header that is not contained in BTC lightclient, then submit since this header
	startPoint = -1
	for i, header := range headers {
		blockHash := header.BlockHash()
		contained, err = r.babylonClient.QueryContainsBlock(&blockHash)
		if err != nil {
			return err
		}
		if !contained {
			startPoint = i
			break
		}
	}

	// all headers are duplicated, submit nothing
	if startPoint == -1 {
		return nil
	}

	headersToSubmit := headers[startPoint:]

	// submit since this header
	// TODO: implement retry mechanism in mustSubmitHeader and keep submitHeader as it is
	err = retry.Do(r.retrySleepTime, r.maxRetrySleepTime, func() error {
		var msgs []*btclctypes.MsgInsertHeader
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

	return err
}

func (r *Reporter) extractCkpts(ib *types.IndexedBlock) int {
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

func (r *Reporter) matchAndSubmitCkpts(signer sdk.AccAddress) error {
	var (
		res                  *sdk.TxResponse
		proofs               []*btcctypes.BTCSpvProof
		msgInsertBTCSpvProof *btcctypes.MsgInsertBTCSpvProof
		ckpts                []*types.Ckpt
		err                  error
	)

	// get matched ckpt parts from the ckptCache
	ckpts = r.CheckpointCache.Match()

	if len(ckpts) == 0 {
		log.Debug("Found no matched pair of checkpoint segments in this match attempt")
		return nil
	}

	// for each matched pair, wrap to MsgInsertBTCSpvProof and send to Babylon
	for _, ckpt := range ckpts {
		log.Info("Found a matched pair of checkpoint segments!")

		proofs, err = ckpt.GenSPVProofs()
		if err != nil {
			log.Errorf("Failed to generate SPV proofs: %v", err)
			continue
		}

		msgInsertBTCSpvProof, err = types.NewMsgInsertBTCSpvProof(signer, proofs)
		if err != nil {
			log.Errorf("Failed to generate new MsgInsertBTCSpvProof: %v", err)
			continue
		}

		//TODO implement retry mechanism in mustInsertBTCSpvProof and keep InsertBTCSpvProof as it is
		err = retry.Do(r.retrySleepTime, r.maxRetrySleepTime, func() error {
			res, err = r.babylonClient.InsertBTCSpvProof(msgInsertBTCSpvProof)
			return err
		})
		if err != nil {
			log.Errorf("Failed to insert new MsgInsertBTCSpvProof: %v", err)
			continue
		}

		log.Infof("Successfully submitted MsgInsertBTCSpvProof with response %d", res.Code)
	}

	return nil
}
