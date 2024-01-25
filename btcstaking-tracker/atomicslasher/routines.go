package atomicslasher

import (
	"time"

	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"go.uber.org/zap"
)

// btcDelegationTracker is a routine that periodically polls BTC delegations
// and updates the BTC delegation index
func (as *AtomicSlasher) btcDelegationTracker() {
	defer as.wg.Done()

	ticker := time.NewTicker(as.cfg.CheckDelegationsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			err := as.bbnAdapter.HandleAllBTCDelegations(func(btcDel *bstypes.BTCDelegation) error {
				trackedDel, err := NewTrackedBTCDelegation(btcDel)
				if err != nil {
					return err
				}
				as.btcDelIndex.Add(trackedDel)
				as.metrics.TrackedBTCDelegationsGauge.Inc()
				return nil
			})
			if err != nil {
				as.logger.Error("failed to handle all BTC delegations", zap.Error(err))
			}
		case <-as.quit:
			return
		}
	}
}

// slashingTxTracker is a routine that keeps tracking new BTC blocks and
// filtering out slashing tx and unbonding slashing tx
func (as *AtomicSlasher) slashingTxTracker() {
	defer as.wg.Done()

	blockNotifier, err := as.btcNotifier.RegisterBlockEpochNtfn(nil)
	if err != nil {
		as.logger.Error("failed to register block notifier", zap.Error(err))
		return
	}
	defer blockNotifier.Cancel()

	// TODO: trim the expired/slashed BTC delegations

	for {
		select {
		case blockEpoch, ok := <-blockNotifier.Epochs:
			if !ok {
				return
			}
			// record BTC tip
			as.btcTipHeight.Store(uint32(blockEpoch.Height))
			as.logger.Debug("Received new best btc block", zap.Int32("height", blockEpoch.Height))
			// get full BTC block
			// TODO: ensure the tx witness is retrieved as well
			_, block, err := as.btcClient.GetBlockByHash(blockEpoch.Hash)
			if err != nil {
				as.logger.Error(
					"failed to get block by hash",
					zap.String("block_hash", blockEpoch.Hash.String()),
					zap.Error(err),
				)
				continue
			}
			// filter out slashing tx / unbonding slashing tx, and
			// enqueue them to the slashed BTC delegation channel
			for _, tx := range block.Transactions {
				txHash := tx.TxHash()
				trackedBTCDel, slashingPath := as.btcDelIndex.FindSlashedBTCDelegation(txHash)
				if trackedBTCDel != nil {
					// this tx is slashing tx
					slashingTxInfo := NewSlashingTxInfo(slashingPath, trackedBTCDel.StakingTxHash, tx)
					as.slashingTxChan <- slashingTxInfo
				}
			}
		case <-as.quit:
			return
		}
	}
}

// selectiveSlashingReporter is a routine that reports finality providers who
// launch selective slashing to Babylon and slashing enforcer routine
func (as *AtomicSlasher) selectiveSlashingReporter() {
	defer as.wg.Done()

	ctx, cancel := as.quitContext()
	bsParams, err := as.bbnAdapter.BTCStakingParams(ctx)
	cancel()
	if err != nil {
		as.logger.Fatal("failed to get BTC staking module parameters", zap.Error(err))
	}

	for {
		select {
		case slashingTxInfo, ok := <-as.slashingTxChan:
			if !ok {
				return
			}
			// get delegation
			stakingTxHash := slashingTxInfo.StakingTxHash
			stakingTxHashStr := stakingTxHash.String()
			ctx, cancel := as.quitContext()
			btcDelResp, err := as.bbnAdapter.BTCDelegation(ctx, stakingTxHashStr)
			cancel()
			if err != nil {
				as.logger.Error(
					"failed to get BTC delegation",
					zap.String("staking_tx_hash", stakingTxHashStr),
					zap.Error(err),
				)
				continue
			}
			// get covenant Schnorr signature in tx
			witnessStack := slashingTxInfo.SlashingMsgTx.TxIn[0].Witness
			covSigMap, fpIdx, fpPK, err := parseSlashingTxWitness(
				witnessStack,
				bsParams.CovenantPks,
				btcDelResp.FpBtcPkList,
			)
			if err != nil {
				as.logger.Error(
					"failed to parse slashing tx witness",
					zap.String("staking_tx_hash", stakingTxHashStr),
					zap.Error(err),
				)
				continue
			}

			// skip if the finality provider is already slashed
			ctx, cancel = as.quitContext()
			isSlashed, err := as.bbnAdapter.IsFPSlashed(ctx, fpPK)
			if err != nil {
				as.logger.Error(
					"failed to query slashing status of finality provider",
					zap.String("staking_tx_hash", stakingTxHashStr),
					zap.String("fp_pk", fpPK.MarshalHex()),
					zap.Error(err),
				)
				continue
			}
			cancel()
			if isSlashed {
				as.logger.Info(
					"the finality provider hosting BTC delegation is already slashed",
					zap.String("staking_tx_hash", stakingTxHashStr),
					zap.String("fp_pk", fpPK.MarshalHex()),
					zap.Error(err),
				)
				continue
			}

			// try to extract this finlaity provider's SK from the list of
			// covenant adaptor signatures and covenant Schnorr signatures
			var fpSK *btcec.PrivateKey
			if slashingTxInfo.IsSlashStakingTx() {
				fpSK, err = tryExtractFPSK(covSigMap, fpIdx, fpPK, btcDelResp.CovenantSigs)
			} else {
				covUnbondingSlashingSigList := btcDelResp.UndelegationInfo.CovenantSlashingSigs
				fpSK, err = tryExtractFPSK(covSigMap, fpIdx, fpPK, covUnbondingSlashingSigList)
			}
			if err != nil {
				// TODO: this implies that all signed covenant members collude with
				// the finality provider. Decide what to do in this case
				as.logger.Error(
					"failed to extract finality provider SK from covenant adaptor signatures and Schnorr signatures",
					zap.String("staking_tx_hash", stakingTxHashStr),
					zap.Error(err),
				)
				continue
			}

			// report selective slashing to Babylon
			ctx, cancel = as.quitContext()
			if err := as.bbnAdapter.ReportSelectiveSlashing(ctx, stakingTxHashStr, fpSK); err != nil {
				// TODO: this implies that all signed covenant members collude with
				// the finality provider. Decide what to do in this case
				as.logger.Error(
					"failed to report a selective slashing finality provider",
					zap.String("staking_tx_hash", stakingTxHashStr),
					zap.Error(err),
				)
			}
			cancel()

			// report to slashing enforcer routine who will slash all
			// the BTC delegations under this finality provider
			as.slashedFPSKChan <- fpSK

			// stop tracking the delegations under this finality provider
			as.btcDelIndex.Remove(stakingTxHash)

		case <-as.quit:
			return
		}
	}
}
