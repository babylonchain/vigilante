package btcslasher

import (
	btcctypes "github.com/babylonchain/babylon/x/btccheckpoint/types"
	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	ftypes "github.com/babylonchain/babylon/x/finality/types"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/cosmos/cosmos-sdk/types/query"
)

type BabylonQueryClient interface {
	BTCCheckpointParams() (*btcctypes.QueryParamsResponse, error)
	BTCStakingParams() (*bstypes.QueryParamsResponse, error)
	BTCValidatorDelegations(valBtcPkHex string, pagination *query.PageRequest) (*bstypes.QueryBTCValidatorDelegationsResponse, error)
	ListEvidences(startHeight uint64, pagination *query.PageRequest) (*ftypes.QueryListEvidencesResponse, error)
	Subscribe(subscriber, query string, outCapacity ...int) (out <-chan coretypes.ResultEvent, err error)
	Unsubscribe(subscriber, query string) error
}
