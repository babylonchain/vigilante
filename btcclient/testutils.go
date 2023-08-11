package btcclient

import (
	"time"

	"github.com/babylonchain/vigilante/config"
	"github.com/babylonchain/vigilante/netparams"
	"github.com/babylonchain/vigilante/types"
	"github.com/btcsuite/btcd/rpcclient"
)

func NewTestClientWithWsSubscriber(rpcClient *rpcclient.Client, cfg *config.BTCConfig, retrySleepTime time.Duration, maxRetrySleepTime time.Duration, blockEventChan chan *types.BlockEvent) (*Client, error) {
	net, err := netparams.GetBTCParams(cfg.NetParams)
	if err != nil {
		return nil, err
	}
	return &Client{
		Client:            rpcClient,
		Params:            net,
		Cfg:               cfg,
		retrySleepTime:    retrySleepTime,
		maxRetrySleepTime: maxRetrySleepTime,
		blockEventChan:    blockEventChan,
	}, nil
}
