package btcslasher

import (
	"fmt"

	bbn "github.com/babylonchain/babylon/types"
	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/wire"
	"github.com/cosmos/cosmos-sdk/types/query"
)

const (
	defaultPaginationLimit = 100
)

func (bs *BTCSlasher) getAllSlashableBTCDelegations(valBTCPK *bbn.BIP340PubKey) ([]*bstypes.BTCDelegation, error) {
	wValue := bs.btcFinalizationTimeout
	_, btcTipHeight, err := bs.BTCClient.GetBestBlock()
	if err != nil {
		return nil, err
	}

	slashableDels := []*bstypes.BTCDelegation{}

	// get all slashable BTC delegations, i.e., BTC delegations whose timelock is not expired yet
	pagination := query.PageRequest{Limit: defaultPaginationLimit}
	for {
		resp, err := bs.BBNQuerier.BTCValidatorDelegations(valBTCPK.MarshalHex(), &pagination)
		if err != nil {
			return nil, fmt.Errorf("failed to get BTC delegations under BTC validator %s: %w", valBTCPK.MarshalHex(), err)
		}
		for _, dels := range resp.BtcDelegatorDelegations {
			for i, del := range dels.Dels {
				// filter out all BTC delegations whose timelock is not expired in BTC yet
				// TODO: if we have anytime unbonding, we need to further check if the BTC
				// delegation has submitted unbonding tx on BTC or not
				if del.StartHeight <= btcTipHeight && btcTipHeight <= del.EndHeight+wValue {
					// avoid using del which changes over the iterations
					slashableDels = append(slashableDels, dels.Dels[i])
				}
			}
		}
		if resp.Pagination == nil || resp.Pagination.NextKey == nil {
			break
		}
		pagination.Key = resp.Pagination.NextKey
	}

	return slashableDels, nil
}

func (bs *BTCSlasher) buildSlashingTxWithWitness(
	sk *btcec.PrivateKey,
	del *bstypes.BTCDelegation,
) (*wire.MsgTx, error) {
	stakingMsgTx, err := del.StakingTx.ToMsgTx()
	if err != nil {
		// this can only be a programming error in Babylon side
		return nil, fmt.Errorf("failed to get BTC delegations under BTC validator %s: %w", del.ValBtcPk.MarshalHex(), err)
	}
	stakingScript := del.StakingTx.StakingScript
	valSig, err := del.SlashingTx.Sign(stakingMsgTx, stakingScript, sk, bs.netParams)
	if err != nil {
		// this can only be a programming error in Babylon side
		return nil, fmt.Errorf("failed to sign slashing tx for the BTC validator: %w", err)
	}

	// assemble witness for slashing tx
	slashingMsgTxWithWitness, err := del.SlashingTx.ToMsgTxWithWitness(del.StakingTx, valSig, del.DelegatorSig, del.JurySig)
	if err != nil {
		// this can only be a programming error in Babylon side
		return nil, fmt.Errorf("failed to assemble witness for slashing tx: %v", err)
	}

	return slashingMsgTxWithWitness, nil
}
