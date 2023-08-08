package config

import (
	"errors"

	"github.com/babylonchain/vigilante/types"
)

const (
	DefaultCheckpointCacheMaxEntries = 100
	DefaultPollingIntervalSeconds    = 60   // in seconds
	DefaultResendIntervalSeconds     = 1800 // 30 minutes
	DefaultResubmitFeeMultiplier     = 1.1
)

// SubmitterConfig defines configuration for the gRPC-web server.
type SubmitterConfig struct {
	// NetParams defines the BTC network params, which should be mainnet|testnet|simnet|signet
	NetParams string `mapstructure:"netparams"`
	// BufferSize defines the number of raw checkpoints stored in the buffer
	BufferSize uint `mapstructure:"buffer-size"`
	// ResubmitFeeMultiplier is used to multiply the estimated the bumped fee in resubmission
	ResubmitFeeMultiplier float64 `mapstructure:"resubmit-fee-multiplier"`
	// PollingIntervalSeconds defines the intervals (in seconds) between each polling of Babylon checkpoints
	PollingIntervalSeconds uint `mapstructure:"polling-interval-seconds"`
	// ResendIntervalSeconds defines the time (in seconds) which the submitter awaits
	// before resubmitting checkpoints to BTC
	ResendIntervalSeconds uint `mapstructure:"resend-interval-seconds"`
}

func (cfg *SubmitterConfig) Validate() error {
	if _, ok := types.GetValidNetParams()[cfg.NetParams]; !ok {
		return errors.New("invalid net params")
	}

	if cfg.ResubmitFeeMultiplier <= 0 {
		return errors.New("invalid resubmit-fee-multiplier, should be more than 0")
	}

	return nil
}

func DefaultSubmitterConfig() SubmitterConfig {
	return SubmitterConfig{
		NetParams:              types.BtcSimnet.String(),
		BufferSize:             DefaultCheckpointCacheMaxEntries,
		ResubmitFeeMultiplier:  DefaultResubmitFeeMultiplier,
		PollingIntervalSeconds: DefaultPollingIntervalSeconds,
		ResendIntervalSeconds:  DefaultResendIntervalSeconds,
	}
}
