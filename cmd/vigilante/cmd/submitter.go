package cmd

import (
	"fmt"

	bbnqccfg "github.com/babylonchain/babylon/client/config"
	bbnqc "github.com/babylonchain/babylon/client/query"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/spf13/cobra"

	"github.com/babylonchain/vigilante/btcclient"
	"github.com/babylonchain/vigilante/config"
	"github.com/babylonchain/vigilante/metrics"
	"github.com/babylonchain/vigilante/rpcserver"
	"github.com/babylonchain/vigilante/submitter"
)

// GetSubmitterCmd returns the CLI commands for the submitter
func GetSubmitterCmd() *cobra.Command {
	var cfgFile = ""
	// Group epoching queries under a subcommand
	cmd := &cobra.Command{
		Use:   "submitter",
		Short: "Vigilant submitter",
		Run: func(_ *cobra.Command, _ []string) {
			// get the config from the given file or the default file
			cfg, err := config.New(cfgFile)
			if err != nil {
				panic(fmt.Errorf("failed to load config: %w", err))
			}
			rootLogger, err := cfg.CreateLogger()
			if err != nil {
				panic(fmt.Errorf("failed to create logger: %w", err))
			}

			// create BTC wallet and connect to BTC server
			btcWallet, err := btcclient.NewWallet(&cfg.BTC, rootLogger)
			if err != nil {
				panic(fmt.Errorf("failed to open BTC client: %w", err))
			}

			// create Babylon query client
			queryCfg := &bbnqccfg.BabylonQueryConfig{
				RPCAddr: cfg.Babylon.RPCAddr,
				Timeout: cfg.Babylon.Timeout,
			}
			err = queryCfg.Validate()
			if err != nil {
				panic(fmt.Errorf("invalid config for the query client: %w", err))
			}
			queryClient, err := bbnqc.New(queryCfg)
			if err != nil {
				panic(fmt.Errorf("failed to create babylon query client: %w", err))
			}

			// get submitter address
			submitterAddr, err := sdk.AccAddressFromBech32(cfg.Babylon.SubmitterAddress)
			if err != nil {
				panic(fmt.Errorf("invalid submitter address from config: %w", err))
			}

			// register submitter metrics
			submitterMetrics := metrics.NewSubmitterMetrics()

			// create submitter
			vigilantSubmitter, err := submitter.New(
				&cfg.Submitter,
				rootLogger,
				btcWallet,
				queryClient,
				submitterAddr,
				cfg.Common.RetrySleepTime,
				cfg.Common.MaxRetrySleepTime,
				submitterMetrics,
			)
			if err != nil {
				panic(fmt.Errorf("failed to create vigilante submitter: %w", err))
			}

			// create RPC server
			server, err := rpcserver.New(&cfg.GRPC, rootLogger, vigilantSubmitter, nil, nil, nil)
			if err != nil {
				panic(fmt.Errorf("failed to create submitter's RPC server: %w", err))
			}

			// start submitter and sync
			vigilantSubmitter.Start()

			// start RPC server
			server.Start()

			// start Prometheus metrics server
			addr := fmt.Sprintf("%s:%d", cfg.Metrics.Host, cfg.Metrics.ServerPort)
			metrics.Start(addr, submitterMetrics.Registry)

			// SIGINT handling stuff
			addInterruptHandler(func() {
				// TODO: Does this need to wait for the grpc server to finish up any requests?
				rootLogger.Info("Stopping RPC server...")
				server.Stop()
				rootLogger.Info("RPC server shutdown")
			})
			addInterruptHandler(func() {
				rootLogger.Info("Stopping submitter...")
				vigilantSubmitter.Stop()
				rootLogger.Info("Submitter shutdown")
			})

			<-interruptHandlersDone
			rootLogger.Info("Shutdown complete")
		},
	}
	cmd.Flags().StringVar(&cfgFile, "config", config.DefaultConfigFile(), "config file")
	return cmd
}
