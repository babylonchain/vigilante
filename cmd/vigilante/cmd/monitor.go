package cmd

import (
	"fmt"

	bbnqccfg "github.com/babylonchain/rpc-client/config"
	bbnqc "github.com/babylonchain/rpc-client/query"
	"github.com/spf13/cobra"

	"github.com/babylonchain/vigilante/btcclient"
	"github.com/babylonchain/vigilante/config"
	"github.com/babylonchain/vigilante/metrics"
	"github.com/babylonchain/vigilante/monitor"
	"github.com/babylonchain/vigilante/rpcserver"
	"github.com/babylonchain/vigilante/types"
)

const (
	genesisFileNameFlag    = "genesis"
	GenesisFileNameDefault = "genesis.json"
)

// GetMonitorCmd returns the CLI commands for the monitor
func GetMonitorCmd() *cobra.Command {
	var genesisFile string
	var cfgFile = ""
	// Group monitor queries under a subcommand
	cmd := &cobra.Command{
		Use:   "monitor",
		Short: "Vigilante monitor constantly checks the consistency between the Babylon node and BTC and detects censorship of BTC checkpoints",
		Run: func(_ *cobra.Command, _ []string) {
			var (
				err              error
				cfg              config.Config
				btcClient        *btcclient.Client
				bbnQueryClient   *bbnqc.QueryClient
				vigilanteMonitor *monitor.Monitor
				server           *rpcserver.Server
			)

			// get the config from the given file or the default file
			cfg, err = config.New(cfgFile)
			if err != nil {
				panic(fmt.Errorf("failed to load config: %w", err))
			}

			// create Babylon query client. Note that requests from Babylon client are ad hoc
			queryCfg := &bbnqccfg.BabylonQueryConfig{
				RPCAddr: cfg.Babylon.RPCAddr,
				Timeout: cfg.Babylon.Timeout,
			}
			if err := queryCfg.Validate(); err != nil {
				panic(fmt.Errorf("invalid config for query client: %w", err))
			}
			bbnQueryClient, err = bbnqc.New(queryCfg)
			if err != nil {
				panic(fmt.Errorf("failed to create babylon query client: %w", err))
			}

			// create BTC client and connect to BTC server
			// Note that monitor needs to subscribe to new BTC blocks
			btcClient, err = btcclient.NewWithBlockSubscriber(&cfg.BTC, cfg.Common.RetrySleepTime, cfg.Common.MaxRetrySleepTime)
			if err != nil {
				panic(fmt.Errorf("failed to open BTC client: %w", err))
			}
			genesisInfo, err := types.GetGenesisInfoFromFile(genesisFile)
			if err != nil {
				panic(fmt.Errorf("failed to read genesis file: %w", err))
			}

			// register monitor metrics
			monitorMetrics := metrics.NewMonitorMetrics()

			// create monitor
			vigilanteMonitor, err = monitor.New(&cfg.Monitor, genesisInfo, cfg.BTC.NetParams, bbnQueryClient, btcClient, monitorMetrics)
			if err != nil {
				panic(fmt.Errorf("failed to create vigilante monitor: %w", err))
			}
			// create RPC server
			server, err = rpcserver.New(&cfg.GRPC, nil, nil, vigilanteMonitor)
			if err != nil {
				panic(fmt.Errorf("failed to create monitor's RPC server: %w", err))
			}

			go vigilanteMonitor.Start()

			// start RPC server
			server.Start()
			// start Prometheus metrics server
			addr := fmt.Sprintf("%s:%d", cfg.Metrics.Host, cfg.Metrics.ServerPort)
			metrics.Start(addr, monitorMetrics.Registry)

			// SIGINT handling stuff
			addInterruptHandler(func() {
				// TODO: Does this need to wait for the grpc server to finish up any requests?
				log.Info("Stopping RPC server...")
				server.Stop()
				log.Info("RPC server shutdown")
			})
			addInterruptHandler(func() {
				log.Info("Stopping monitor...")
				vigilanteMonitor.Stop()
				log.Info("Monitor shutdown")
			})

			<-interruptHandlersDone
			log.Info("Shutdown complete")
		},
	}
	cmd.Flags().StringVar(&genesisFile, genesisFileNameFlag, GenesisFileNameDefault, "genesis file")
	cmd.Flags().StringVar(&cfgFile, "config", config.DefaultConfigFile(), "config file")
	return cmd
}
