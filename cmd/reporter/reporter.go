package reporter

import (
	"github.com/babylonchain/vigilante/babylonclient"
	"github.com/babylonchain/vigilante/btcclient"
	"github.com/babylonchain/vigilante/cmd/utils"
	"github.com/babylonchain/vigilante/config"
	vlog "github.com/babylonchain/vigilante/log"
	"github.com/babylonchain/vigilante/metrics"
	"github.com/babylonchain/vigilante/reporter"
	"github.com/babylonchain/vigilante/rpcserver"
	"github.com/spf13/cobra"
	"time"
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
		err               error
		cfg               config.Config
		btcClient         *btcclient.Client
		babylonClient     *babylonclient.Client
		vigilantReporter  *reporter.Reporter
		server            *rpcserver.Server
		retrySleepTime    time.Duration
		maxRetrySleepTime time.Duration
	)

	// get the config from the given file or the default file
	cfg, err = config.New(cfgFile)
	if err != nil {
		panic(err)
	}

	// apply the flags from CLI
	if len(babylonKeyDir) != 0 {
		cfg.Babylon.KeyDirectory = babylonKeyDir
	}

	if retrySleepTime, err = time.ParseDuration(cfg.Common.RetrySleepTime); err != nil {
		panic(err)
	}

	if maxRetrySleepTime, err = time.ParseDuration(cfg.Common.MaxRetrySleepTime); err != nil {
		panic(err)
	}

	// create BTC client and connect to BTC server
	// Note that vigilant reporter needs to subscribe to new BTC blocks
	if cfg.BTC.Polling {
		btcClient, err = btcclient.NewWithBlockPoller(&cfg.BTC, retrySleepTime, maxRetrySleepTime)
	} else {
		btcClient, err = btcclient.NewWithBlockSubscriber(&cfg.BTC, retrySleepTime, maxRetrySleepTime)
	}
	if err != nil {
		panic(err)
	}

	// create Babylon client. Note that requests from Babylon client are ad hoc
	babylonClient, err = babylonclient.New(&cfg.Babylon, retrySleepTime, maxRetrySleepTime)
	if err != nil {
		panic(err)
	}
	// create reporter
	vigilantReporter, err = reporter.New(&cfg.Reporter, btcClient, babylonClient, retrySleepTime, maxRetrySleepTime)
	if err != nil {
		panic(err)
	}
	// create RPC server
	server, err = rpcserver.New(&cfg.GRPC, nil, vigilantReporter)
	if err != nil {
		panic(err)
	}

	// bootstrapping
	vigilantReporter.Init()
	// start normal-case execution
	vigilantReporter.Start()

	// start RPC server
	server.Start()
	// start Prometheus metrics server
	metrics.Start()

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
