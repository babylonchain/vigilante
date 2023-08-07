package submitter

import (
	"github.com/babylonchain/vigilante/submitter/poller"

	btcctypes "github.com/babylonchain/babylon/x/btccheckpoint/types"
)

type BabylonQueryClient interface {
	poller.BabylonQueryClient
	BTCCheckpointParams() (*btcctypes.QueryParamsResponse, error)
}
