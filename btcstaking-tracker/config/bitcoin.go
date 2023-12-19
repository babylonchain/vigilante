package config

import (
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/babylonchain/vigilante/config"
	"github.com/babylonchain/vigilante/types"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcwallet/chain"
	"github.com/lightningnetwork/lnd/blockcache"
	"github.com/lightningnetwork/lnd/chainntnfs"
	"github.com/lightningnetwork/lnd/chainntnfs/bitcoindnotify"
	"github.com/lightningnetwork/lnd/chainntnfs/btcdnotify"
)

type Btcd struct {
	RPCHost        string
	RPCUser        string
	RPCPass        string
	RPCCert        string
	RawCert        string
	DisableTLS     bool
	BlockCacheSize uint64
}

type Bitcoind struct {
	RPCHost              string
	RPCUser              string
	RPCPass              string
	ZMQPubRawBlock       string
	ZMQPubRawTx          string
	ZMQReadDeadline      time.Duration
	EstimateMode         string
	PrunedNodeMaxPeers   int
	RPCPolling           bool
	BlockPollingInterval time.Duration
	TxPollingInterval    time.Duration
	BlockCacheSize       uint64
}

func DefaultBitcoindConfig() Bitcoind {
	return Bitcoind{
		RPCHost:              config.DefaultRpcBtcNodeHost,
		RPCUser:              config.DefaultBtcNodeRpcUser,
		RPCPass:              config.DefaultBtcNodeRpcPass,
		RPCPolling:           false,
		BlockPollingInterval: 30 * time.Second,
		TxPollingInterval:    30 * time.Second,
		EstimateMode:         config.DefaultBtcNodeEstimateMode,
		BlockCacheSize:       config.DefaultBtcblockCacheSize,
		ZMQPubRawBlock:       config.DefaultZmqBlockEndpoint,
		ZMQPubRawTx:          config.DefaultZmqTxEndpoint,
		ZMQReadDeadline:      30 * time.Second,
	}
}

type BtcNodeBackendConfig struct {
	Btcd              *Btcd
	Bitcoind          *Bitcoind
	ActiveNodeBackend types.SupportedBtcBackend
}

func CfgToBtcNodeBackendConfig(cfg config.BTCConfig, rawCert string) *BtcNodeBackendConfig {
	switch cfg.BtcBackend {
	case types.Bitcoind:
		defaultBitcoindCfg := DefaultBitcoindConfig()
		// Re-rewrite defaults by values from global cfg
		defaultBitcoindCfg.RPCHost = cfg.Endpoint
		defaultBitcoindCfg.RPCUser = cfg.Username
		defaultBitcoindCfg.RPCPass = cfg.Password
		defaultBitcoindCfg.ZMQPubRawBlock = cfg.ZmqBlockEndpoint
		defaultBitcoindCfg.ZMQPubRawTx = cfg.ZmqTxEndpoint
		defaultBitcoindCfg.EstimateMode = cfg.EstimateMode

		return &BtcNodeBackendConfig{
			ActiveNodeBackend: types.Bitcoind,
			Bitcoind:          &defaultBitcoindCfg,
		}

	case types.Btcd:
		return &BtcNodeBackendConfig{
			ActiveNodeBackend: types.Btcd,
			Btcd: &Btcd{
				RPCHost:    cfg.Endpoint,
				RPCUser:    cfg.Username,
				RPCPass:    cfg.Password,
				RPCCert:    cfg.CAFile,
				RawCert:    rawCert,
				DisableTLS: cfg.DisableClientTLS,
				// TODO: Make block cache size configurable. Note: this is value is in bytes.
				BlockCacheSize: config.DefaultBtcblockCacheSize,
			},
		}
	default:
		panic(fmt.Sprintf("unknown btc backend: %v", cfg.BtcBackend))
	}
}

type NodeBackend struct {
	chainntnfs.ChainNotifier
}

type HintCache interface {
	chainntnfs.SpendHintCache
	chainntnfs.ConfirmHintCache
}

// type for disabled hint cache
// TODO: Determine if we need hint cache backed up by database which is provided
// by lnd.
type EmptyHintCache struct{}

var _ HintCache = (*EmptyHintCache)(nil)

func (c *EmptyHintCache) CommitSpendHint(height uint32, spendRequests ...chainntnfs.SpendRequest) error {
	return nil
}
func (c *EmptyHintCache) QuerySpendHint(spendRequest chainntnfs.SpendRequest) (uint32, error) {
	return 0, nil
}
func (c *EmptyHintCache) PurgeSpendHint(spendRequests ...chainntnfs.SpendRequest) error {
	return nil
}

func (c *EmptyHintCache) CommitConfirmHint(height uint32, confRequests ...chainntnfs.ConfRequest) error {
	return nil
}
func (c *EmptyHintCache) QueryConfirmHint(confRequest chainntnfs.ConfRequest) (uint32, error) {
	return 0, nil
}
func (c *EmptyHintCache) PurgeConfirmHint(confRequests ...chainntnfs.ConfRequest) error {
	return nil
}

// // TODO  This should be moved to a more appropriate place, most probably to config
// // and be connected to validation of rpc host/port.
// // According to chain.BitcoindConfig docs it should also support tor if node backend
// // works over tor.
func BuildDialer(rpcHost string) func(string) (net.Conn, error) {
	return func(addr string) (net.Conn, error) {
		return net.Dial("tcp", rpcHost)
	}
}

func NewNodeBackend(
	cfg *BtcNodeBackendConfig,
	params *chaincfg.Params,
	hintCache HintCache,
) (*NodeBackend, error) {
	switch cfg.ActiveNodeBackend {
	case types.Bitcoind:
		bitcoindCfg := &chain.BitcoindConfig{
			ChainParams:        params,
			Host:               cfg.Bitcoind.RPCHost,
			User:               cfg.Bitcoind.RPCUser,
			Pass:               cfg.Bitcoind.RPCPass,
			Dialer:             BuildDialer(cfg.Bitcoind.RPCHost),
			PrunedModeMaxPeers: cfg.Bitcoind.PrunedNodeMaxPeers,
		}

		if cfg.Bitcoind.RPCPolling {
			bitcoindCfg.PollingConfig = &chain.PollingConfig{
				BlockPollingInterval:    cfg.Bitcoind.BlockPollingInterval,
				TxPollingInterval:       cfg.Bitcoind.TxPollingInterval,
				TxPollingIntervalJitter: config.DefaultTxPollingJitter,
			}
		} else {
			bitcoindCfg.ZMQConfig = &chain.ZMQConfig{
				ZMQBlockHost:           cfg.Bitcoind.ZMQPubRawBlock,
				ZMQTxHost:              cfg.Bitcoind.ZMQPubRawTx,
				ZMQReadDeadline:        cfg.Bitcoind.ZMQReadDeadline,
				MempoolPollingInterval: cfg.Bitcoind.TxPollingInterval,
				PollingIntervalJitter:  config.DefaultTxPollingJitter,
			}
		}

		bitcoindConn, err := chain.NewBitcoindConn(bitcoindCfg)
		if err != nil {
			return nil, err
		}

		if err := bitcoindConn.Start(); err != nil {
			return nil, fmt.Errorf("unable to connect to "+
				"bitcoind: %v", err)
		}

		chainNotifier := bitcoindnotify.New(
			bitcoindConn, params, hintCache,
			hintCache, blockcache.NewBlockCache(cfg.Bitcoind.BlockCacheSize),
		)

		return &NodeBackend{
			ChainNotifier: chainNotifier,
		}, nil

	case types.Btcd:
		btcdUser := cfg.Btcd.RPCUser
		btcdPass := cfg.Btcd.RPCPass
		btcdHost := cfg.Btcd.RPCHost

		var certs []byte
		if !cfg.Btcd.DisableTLS {
			if cfg.Btcd.RawCert != "" {
				decoded, err := hex.DecodeString(cfg.Btcd.RawCert)
				if err != nil {
					return nil, fmt.Errorf("error decoding btcd cert: %v", err)
				}
				certs = decoded

			} else {
				certificates, err := os.ReadFile(cfg.Btcd.RPCCert)
				if err != nil {
					return nil, fmt.Errorf("error reading btcd cert file: %v", err)
				}
				certs = certificates
			}
		}

		rpcConfig := &rpcclient.ConnConfig{
			Host:                 btcdHost,
			Endpoint:             "ws",
			User:                 btcdUser,
			Pass:                 btcdPass,
			Certificates:         certs,
			DisableTLS:           cfg.Btcd.DisableTLS,
			DisableConnectOnNew:  true,
			DisableAutoReconnect: false,
		}

		chainNotifier, err := btcdnotify.New(
			rpcConfig, params, hintCache,
			hintCache, blockcache.NewBlockCache(cfg.Btcd.BlockCacheSize),
		)

		if err != nil {
			return nil, err
		}

		return &NodeBackend{
			ChainNotifier: chainNotifier,
		}, nil

	default:
		return nil, fmt.Errorf("unknown node backend: %v", cfg.ActiveNodeBackend)
	}
}
