package relayer

import (
	"fmt"

	"github.com/btcsuite/btcd/rpcclient"
	"github.com/lightningnetwork/lnd/lnwallet/chainfee"

	"github.com/babylonchain/vigilante/config"
	"github.com/babylonchain/vigilante/log"
	"github.com/babylonchain/vigilante/netparams"
	"github.com/babylonchain/vigilante/types"
)

// NewFeeEstimator creates a fee estimator based on the given backend
// currently, we only support bitcoind and btcd
func NewFeeEstimator(cfg *config.BTCConfig) (chainfee.Estimator, error) {
	params := netparams.GetBTCParams(cfg.NetParams)

	connCfg := &rpcclient.ConnConfig{}
	var est chainfee.Estimator
	switch cfg.BtcBackend {
	case types.Bitcoind:
		connCfg = &rpcclient.ConnConfig{
			// this will work with node loaded with multiple wallets
			Host:         cfg.Endpoint + "/wallet/" + cfg.WalletName,
			HTTPPostMode: true,
			User:         cfg.Username,
			Pass:         cfg.Password,
			DisableTLS:   cfg.DisableClientTLS,
			Params:       params.Name,
		}
		bitcoindEst, err := chainfee.NewBitcoindEstimator(
			*connCfg, cfg.EstimateMode, cfg.DefaultFee.FeePerKWeight(),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create fee estimator for %s backend: %w", types.Bitcoind, err)
		}
		est = bitcoindEst
	case types.Btcd:
		connCfg = &rpcclient.ConnConfig{
			Host:         cfg.WalletEndpoint,
			Endpoint:     "ws", // websocket
			User:         cfg.Username,
			Pass:         cfg.Password,
			DisableTLS:   cfg.DisableClientTLS,
			Params:       params.Name,
			Certificates: cfg.ReadWalletCAFile(),
		}
		btcdEst, err := chainfee.NewBtcdEstimator(
			*connCfg, cfg.DefaultFee.FeePerKWeight(),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create fee estimator for %s backend: %w", types.Btcd, err)
		}
		est = btcdEst
	default:
		return nil, fmt.Errorf("unsupported backend for fee estimator")
	}

	if err := est.Start(); err != nil {
		return nil, fmt.Errorf("failed to initiate the fee estimator for %s backend: %w", cfg.BtcBackend, err)
	}

	log.Logger.Infof("Successfully started fee estimator for %s backend", cfg.BtcBackend)

	return est, nil
}
