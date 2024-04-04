package atomicslasher

import (
	"context"

	"cosmossdk.io/errors"
	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkquerytypes "github.com/cosmos/cosmos-sdk/types/query"
	pv "github.com/cosmos/relayer/v2/relayer/provider"
)

type BabylonClient interface {
	FinalityProvider(fpBtcPkHex string) (*bstypes.QueryFinalityProviderResponse, error)
	BTCDelegations(status bstypes.BTCDelegationStatus, pagination *sdkquerytypes.PageRequest) (*bstypes.QueryBTCDelegationsResponse, error)
	BTCDelegation(stakingTxHashHex string) (*bstypes.QueryBTCDelegationResponse, error)
	BTCStakingParamsByVersion(version uint32) (*bstypes.QueryParamsByVersionResponse, error)
	ReliablySendMsg(ctx context.Context, msg sdk.Msg, expectedErrors []*errors.Error, unrecoverableErrors []*errors.Error) (*pv.RelayerTxResponse, error)
	MustGetAddr() string
}
