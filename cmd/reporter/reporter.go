package reporter

import (
	"fmt"

	bbnclient "github.com/babylonchain/rpc-client/client"
	"github.com/spf13/cobra"

	"github.com/babylonchain/vigilante/btcclient"
	"github.com/babylonchain/vigilante/cmd/utils"
	"github.com/babylonchain/vigilante/config"
	vlog "github.com/babylonchain/vigilante/log"
	"github.com/babylonchain/vigilante/metrics"
	"github.com/babylonchain/vigilante/reporter"
	"github.com/babylonchain/vigilante/rpcserver"
)

var (
	log           = vlog.Logger.WithField("module", "cmd")
	cfgFile       = ""
	babylonKeyDir = ""
)

func addFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&cfgFile, "config", config.DefaultConfigFile(), "config file")
	cmd.Flags().StringVar(&babylonKeyDir, "babylon-key", "", "Directory of the Babylon key")
}

// GetCmd returns the cli query commands for this module
func GetCmd() *cobra.Command {
	// Group epoching queries under a subcommand
	cmd := &cobra.Command{
		Use:   "reporter",
		Short: "Vigilant reporter",
		Run:   cmdFunc,
	}
	addFlags(cmd)
	return cmd
}

func cmdFunc(cmd *cobra.Command, args []string) {
	var (
		err              error
		cfg              config.Config
		btcClient        *btcclient.Client
		babylonClient    *bbnclient.Client
		vigilantReporter *reporter.Reporter
		server           *rpcserver.Server
	)

	// get the config from the given file or the default file
	cfg, err = config.New(cfgFile)
	if err != nil {
		panic(fmt.Errorf("failed to load config: %w", err))
	}

	// apply the flags from CLI
	if len(babylonKeyDir) != 0 {
		cfg.Babylon.KeyDirectory = babylonKeyDir
	}

	// create BTC client and connect to BTC server
	// Note that vigilant reporter needs to subscribe to new BTC blocks
	btcClient, err = btcclient.NewWithBlockSubscriber(&cfg.BTC, cfg.Common.RetrySleepTime, cfg.Common.MaxRetrySleepTime)
	if err != nil {
		panic(fmt.Errorf("failed to open BTC client: %w", err))
	}

	// create Babylon client. Note that requests from Babylon client are ad hoc
	babylonClient, err = bbnclient.New(&cfg.Babylon)
	if err != nil {
		panic(fmt.Errorf("failed to open Babylon client: %w", err))
	}

	// register reporter metrics
	reporterMetrics := metrics.NewReporterMetrics()

	// create reporter
	vigilantReporter, err = reporter.New(
		&cfg.Reporter,
		btcClient,
		babylonClient,
		cfg.Common.RetrySleepTime,
		cfg.Common.MaxRetrySleepTime,
		reporterMetrics,
	)
	if err != nil {
		panic(fmt.Errorf("failed to create vigilante reporter: %w", err))
	}

	// create RPC server
	server, err = rpcserver.New(&cfg.GRPC, nil, vigilantReporter, nil)
	if err != nil {
		panic(fmt.Errorf("failed to create reporter's RPC server: %w", err))
	}

	// bootstrapping
	vigilantReporter.Bootstrap(false)

	// start normal-case execution
	vigilantReporter.Start()

	// start RPC server
	server.Start()
	// start Prometheus metrics server
	addr := fmt.Sprintf("%s:%d", cfg.Metrics.Host, cfg.Metrics.ServerPort)
	metrics.Start(addr, reporterMetrics.Registry)

	// SIGINT handling stuff
	utils.AddInterruptHandler(func() {
		// TODO: Does this need to wait for the grpc server to finish up any requests?
		log.Info("Stopping RPC server...")
		server.Stop()
		log.Info("RPC server shutdown")
	})
	utils.AddInterruptHandler(func() {
		log.Info("Stopping reporter...")
		vigilantReporter.Stop()
		log.Info("Reporter shutdown")
	})

	<-utils.InterruptHandlersDone
	log.Info("Shutdown complete")
}
