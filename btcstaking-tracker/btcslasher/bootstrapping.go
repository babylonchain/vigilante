package btcslasher

import (
	"fmt"

	ftypes "github.com/babylonchain/babylon/x/finality/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"github.com/hashicorp/go-multierror"
)

// Bootstrap bootstraps the BTC slasher. Specifically, it checks all evidences
// since the given startHeight to see if any slashing tx is not submitted to Bitcoin.
// If the slashing tx under a finality provider with an equivocation evidence is still
// spendable on Bitcoin, then it will submit it to Bitcoin thus slashing this BTC delegation.
func (bs *BTCSlasher) Bootstrap(startHeight uint64) error {
	bs.logger.Info("start bootstrapping BTC slasher")

	// load module parameters
	if err := bs.LoadParams(); err != nil {
		return err
	}

	// handle all evidences since the given start height, i.e., for each evidence,
	// extract its SK and try to slash all BTC delegations under it
	err := bs.handleAllEvidences(startHeight, func(evidences []*ftypes.Evidence) error {
		var accumulatedErrs error // we use this variable to accumulate errors

		for _, evidence := range evidences {
			fpBTCPK := evidence.FpBtcPk
			fpBTCPKHex := fpBTCPK.MarshalHex()
			bs.logger.Infof("found evidence for finality provider %s at height %d after start height %d", fpBTCPKHex, evidence.BlockHeight, startHeight)

			// extract the SK of the slashed finality provider
			fpBTCSK, err := evidence.ExtractBTCSK()
			if err != nil {
				bs.logger.Errorf("failed to extract BTC SK of the slashed finality provider %s: %v", fpBTCPKHex, err)
				accumulatedErrs = multierror.Append(accumulatedErrs, err)
				continue
			}

			// slash this finality provider's all BTC delegations whose slashing tx's input is still spendable
			// on Bitcoin
			if err := bs.SlashFinalityProvider(fpBTCSK); err != nil {
				bs.logger.Errorf("failed to slash finality provider %s: %v", fpBTCPKHex, err)
				accumulatedErrs = multierror.Append(accumulatedErrs, err)
				continue
			}
		}

		return accumulatedErrs
	})

	if err != nil {
		return fmt.Errorf("failed to bootstrap BTC slasher: %w", err)
	}

	return nil
}

func (bs *BTCSlasher) handleAllEvidences(startHeight uint64, handleFunc func(evidences []*ftypes.Evidence) error) error {
	pagination := query.PageRequest{Limit: defaultPaginationLimit}
	for {
		resp, err := bs.BBNQuerier.ListEvidences(startHeight, &pagination)
		if err != nil {
			return fmt.Errorf("failed to get evidences: %w", err)
		}
		if err := handleFunc(resp.Evidences); err != nil {
			// we should continue getting and handling evidences in subsequent pages
			// rather than return here
			bs.logger.Errorf("failed to handle evidences: %v", err)
		}
		if resp.Pagination == nil || resp.Pagination.NextKey == nil {
			break
		}
		pagination.Key = resp.Pagination.NextKey
	}

	return nil
}
