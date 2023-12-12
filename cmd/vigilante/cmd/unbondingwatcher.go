package cmd

import (
	"fmt"

	bbnclient "github.com/babylonchain/rpc-client/client"
	"github.com/babylonchain/vigilante/config"
	"github.com/babylonchain/vigilante/metrics"
	uw "github.com/babylonchain/vigilante/monitor/unbondingwatcher"
	"github.com/babylonchain/vigilante/netparams"
	"github.com/babylonchain/vigilante/rpcserver"
	"github.com/spf13/cobra"
)

// GetReporterCmd returns the CLI commands for the reporter
func GetUnbondingWatcherCmd() *cobra.Command {
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

			ba := uw.NewBabylonClientAdapter(babylonClient)

			btcCfg := uw.CfgToBtcNodeBackendConfig(
				cfg.BTC,
				"", // we will read certifcates from file
			)
			notifer, err := uw.NewNodeBackend(
				btcCfg,
				btcParams,
				&uw.EmptyHintCache{},
			)

			if err != nil {
				panic(fmt.Errorf("failed to create btc chain notifier: %w", err))
			}

			watcherMetrics := metrics.NewUnbondingWatcherMetrics()

			watcher := uw.NewUnbondingWatcher(
				notifer,
				ba,
				&cfg.UnbondingWatcher,
				rootLogger,
				watcherMetrics,
			)

			// create RPC server
			server, err = rpcserver.New(&cfg.GRPC, rootLogger, nil, nil, nil, watcher)
			if err != nil {
				panic(fmt.Errorf("failed to create reporter's RPC server: %w", err))
			}

			err = notifer.Start()

			if err != nil {
				panic(fmt.Errorf("failed to start btc chain notifier: %w", err))
			}

			err = watcher.Start()

			if err != nil {
				panic(fmt.Errorf("failed to start unbonding watcher: %w", err))
			}

			// start RPC server
			server.Start()
			// start Prometheus metrics server
			addr := fmt.Sprintf("%s:%d", cfg.Metrics.Host, cfg.Metrics.ServerPort)
			metrics.Start(addr, watcherMetrics.Registry)

			// SIGINT handling stuff
			addInterruptHandler(func() {
				// TODO: Does this need to wait for the grpc server to finish up any requests?
				rootLogger.Info("Stopping RPC server...")
				server.Stop()
				rootLogger.Info("RPC server shutdown")
			})
			addInterruptHandler(func() {
				rootLogger.Info("Stopping unbonding watcher...")
				err := watcher.Stop()

				if err != nil {
					panic(fmt.Errorf("failed to stop unbonding watcher: %w", err))
				}

				rootLogger.Info("Unbonding watcher shutdown")
			})
			addInterruptHandler(func() {
				rootLogger.Info("Stopping BTC notifier...")
				err := notifer.Stop()

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
