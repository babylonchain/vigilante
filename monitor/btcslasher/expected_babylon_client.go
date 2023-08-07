package btcslasher

import (
	btcctypes "github.com/babylonchain/babylon/x/btccheckpoint/types"
	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/cosmos/cosmos-sdk/types/query"
)

type BabylonQueryClient interface {
	BTCCheckpointParams() (*btcctypes.QueryParamsResponse, error)
	BTCValidatorDelegations(valBtcPkHex string, pagination *query.PageRequest) (*bstypes.QueryBTCValidatorDelegationsResponse, error)
	Subscribe(subscriber, query string, outCapacity ...int) (out <-chan coretypes.ResultEvent, err error)
	Unsubscribe(subscriber, query string) error
}
