package cmd

import (
	"fmt"

	bbnclient "github.com/babylonchain/rpc-client/client"
	bst "github.com/babylonchain/vigilante/btcstaking-tracker"
	bstcfg "github.com/babylonchain/vigilante/btcstaking-tracker/config"
	bsttypes "github.com/babylonchain/vigilante/btcstaking-tracker/types"
	"github.com/babylonchain/vigilante/config"
	"github.com/babylonchain/vigilante/metrics"
	"github.com/babylonchain/vigilante/netparams"
	"github.com/babylonchain/vigilante/rpcserver"
	"github.com/spf13/cobra"
)

func GetBTCStakingTracker() *cobra.Command {
	var babylonKeyDir string
	var cfgFile = ""

	cmd := &cobra.Command{
		Use:   "uwatcher",
		Short: "Unbonding watcher",
		Run: func(_ *cobra.Command, _ []string) {
			var (
				err           error
				cfg           config.Config
				babylonClient *bbnclient.Client
				server        *rpcserver.Server
			)

			// get the config from the given file or the default file
			cfg, err = config.New(cfgFile)
			if err != nil {
				panic(fmt.Errorf("failed to load config: %w", err))
			}
			rootLogger, err := cfg.CreateLogger()
			if err != nil {
				panic(fmt.Errorf("failed to create logger: %w", err))
			}

			// apply the flags from CLI
			if len(babylonKeyDir) != 0 {
				cfg.Babylon.KeyDirectory = babylonKeyDir
			}

			// create Babylon client. Note that requests from Babylon client are ad hoc
			babylonClient, err = bbnclient.New(&cfg.Babylon, nil)
			if err != nil {
				panic(fmt.Errorf("failed to open Babylon client: %w", err))
			}

			btcParams, err := netparams.GetBTCParams(cfg.BTC.NetParams)
			if err != nil {
				panic(fmt.Errorf("failed to get BTC parameter: %w", err))
			}

			ba := bsttypes.NewBabylonClientAdapter(babylonClient)

			btcCfg := bstcfg.CfgToBtcNodeBackendConfig(
				cfg.BTC,
				"", // we will read certifcates from file
			)
			notifier, err := bstcfg.NewNodeBackend(
				btcCfg,
				btcParams,
				&bstcfg.EmptyHintCache{},
			)

			if err != nil {
				panic(fmt.Errorf("failed to create btc chain notifier: %w", err))
			}

			bsMetrics := metrics.NewBTCStakingTrackerMetrics()

			bstracker := bst.NewBTCSTakingTracker(
				notifier,
				ba,
				&cfg.BTCStakingTracker,
				rootLogger,
				bsMetrics,
			)

			// create RPC server
			server, err = rpcserver.New(&cfg.GRPC, rootLogger, nil, nil, nil, bstracker)
			if err != nil {
				panic(fmt.Errorf("failed to create reporter's RPC server: %w", err))
			}

			err = notifier.Start()

			if err != nil {
				panic(fmt.Errorf("failed to start btc chain notifier: %w", err))
			}

			err = bstracker.Start()

			if err != nil {
				panic(fmt.Errorf("failed to start unbonding watcher: %w", err))
			}

			// start RPC server
			server.Start()
			// start Prometheus metrics server
			addr := fmt.Sprintf("%s:%d", cfg.Metrics.Host, cfg.Metrics.ServerPort)
			metrics.Start(addr, bsMetrics.Registry)

			// SIGINT handling stuff
			addInterruptHandler(func() {
				// TODO: Does this need to wait for the grpc server to finish up any requests?
				rootLogger.Info("Stopping RPC server...")
				server.Stop()
				rootLogger.Info("RPC server shutdown")
			})
			addInterruptHandler(func() {
				rootLogger.Info("Stopping unbonding watcher...")
				err := bstracker.Stop()

				if err != nil {
					panic(fmt.Errorf("failed to stop unbonding watcher: %w", err))
				}

				rootLogger.Info("Unbonding watcher shutdown")
			})
			addInterruptHandler(func() {
				rootLogger.Info("Stopping BTC notifier...")
				err := bstracker.Stop()

				if err != nil {
					panic(fmt.Errorf("failed to stop btc chain notifier: %w", err))
				}

				rootLogger.Info("BTC nofifier shutdown")
			})

			<-interruptHandlersDone
			rootLogger.Info("Shutdown complete")

		},
	}
	cmd.Flags().StringVar(&babylonKeyDir, "babylon-key", "", "Directory of the Babylon key")
	cmd.Flags().StringVar(&cfgFile, "config", config.DefaultConfigFile(), "config file")
	return cmd
}
