package atomicslasher

import (
	"context"
	"fmt"
	"time"

	"cosmossdk.io/errors"
	"github.com/avast/retry-go/v4"
	bbn "github.com/babylonchain/babylon/types"
	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	"github.com/babylonchain/vigilante/config"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/cosmos/cosmos-sdk/types/query"
	"go.uber.org/zap"
)

type BabylonAdapter struct {
	logger            *zap.Logger
	cfg               *config.BTCStakingTrackerConfig
	retrySleepTime    time.Duration
	maxRetrySleepTime time.Duration
	bbnClient         BabylonClient
}

func NewBabylonAdapter(
	logger *zap.Logger,
	cfg *config.BTCStakingTrackerConfig,
	retrySleepTime time.Duration,
	maxRetrySleepTime time.Duration,
	bbnClient BabylonClient,
) *BabylonAdapter {
	return &BabylonAdapter{
		logger:            logger,
		cfg:               cfg,
		retrySleepTime:    retrySleepTime,
		maxRetrySleepTime: maxRetrySleepTime,
		bbnClient:         bbnClient,
	}
}

func (ba *BabylonAdapter) BTCStakingParams(ctx context.Context) (*bstypes.Params, error) {
	var bsParams *bstypes.Params
	err := retry.Do(
		func() error {
			resp, err := ba.bbnClient.BTCStakingParams()
			if err != nil {
				return err
			}
			bsParams = &resp.Params
			return nil
		},
		retry.Context(ctx),
		retry.Delay(ba.retrySleepTime),
		retry.MaxDelay(ba.maxRetrySleepTime),
	)

	return bsParams, err
}

func (ba *BabylonAdapter) BTCDelegation(ctx context.Context, stakingTxHashHex string) (*bstypes.QueryBTCDelegationResponse, error) {
	var resp *bstypes.QueryBTCDelegationResponse
	err := retry.Do(
		func() error {
			resp2, err := ba.bbnClient.BTCDelegation(stakingTxHashHex)
			if err != nil {
				return err
			}
			resp = resp2
			return nil
		},
		retry.Context(ctx),
		retry.Delay(ba.retrySleepTime),
		retry.MaxDelay(ba.maxRetrySleepTime),
	)

	return resp, err
}

// TODO: avoid getting expired BTC delegations
func (ba *BabylonAdapter) HandleAllBTCDelegations(handleFunc func(btcDel *bstypes.BTCDelegation) error) error {
	pagination := query.PageRequest{Limit: ba.cfg.NewDelegationsBatchSize}

	for {
		resp, err := ba.bbnClient.BTCDelegations(bstypes.BTCDelegationStatus_ANY, &pagination)
		if err != nil {
			return fmt.Errorf("failed to get BTC delegations: %w", err)
		}
		for _, btcDel := range resp.BtcDelegations {
			if err := handleFunc(btcDel); err != nil {
				// we should continue getting and handling evidences in subsequent pages
				// rather than return here
				ba.logger.Error("failed to handle BTC delegations", zap.Error(err))
			}
		}
		if resp.Pagination == nil || resp.Pagination.NextKey == nil {
			break
		}
		pagination.Key = resp.Pagination.NextKey
	}

	return nil
}

func (ba *BabylonAdapter) IsFPSlashed(
	ctx context.Context,
	fpBTCPK *bbn.BIP340PubKey,
) (bool, error) {
	fpResp, err := ba.bbnClient.FinalityProvider(fpBTCPK.MarshalHex())
	if err != nil {
		return false, err
	}

	return fpResp.FinalityProvider.IsSlashed(), nil
}

func (ba *BabylonAdapter) ReportSelectiveSlashing(
	ctx context.Context,
	stakingTxHash string,
	fpBTCSK *btcec.PrivateKey,
) error {
	msg := &bstypes.MsgSelectiveSlashingEvidence{
		Signer:           ba.bbnClient.MustGetAddr(),
		StakingTxHash:    stakingTxHash,
		RecoveredFpBtcSk: fpBTCSK.Serialize(),
	}

	// TODO: what are unrecoverable/expected errors?
	_, err := ba.bbnClient.ReliablySendMsg(ctx, msg, []*errors.Error{}, []*errors.Error{})
	return err
}
