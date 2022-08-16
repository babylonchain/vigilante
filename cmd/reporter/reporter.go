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
)

var (
	cfgFile = ""
	log     = vlog.Logger.WithField("module", "cmd")
)

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

func addFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&cfgFile, "config", "", "config file")
}

func cmdFunc(cmd *cobra.Command, args []string) {
	// get the config from the given file, the default file, or generate a default config
	var err error
	var cfg config.Config
	if len(cfgFile) != 0 {
		cfg, err = config.NewFromFile(cfgFile)
	} else {
		cfg, err = config.New()
	}
	if err != nil {
		panic(err)
	}

	// create BTC client and connect to BTC server
	btcClient, err := btcclient.New(&cfg.BTC)
	if err != nil {
		panic(err)
	}
	// create Babylon client. Note that requests from Babylon client are ad hoc
	babylonClient, err := babylonclient.New(&cfg.Babylon)
	if err != nil {
		panic(err)
	}
	// create reporter
	vigilantReporter, err := reporter.New(&cfg.Reporter, btcClient, babylonClient)
	if err != nil {
		panic(err)
	}
	// create RPC server
	server, err := rpcserver.New(&cfg.GRPC, nil, vigilantReporter)
	if err != nil {
		panic(err)
	}

	// start reporter and sync
	vigilantReporter.Start()
	// start RPC server
	server.Start()
	// start Prometheus metrics server
	metrics.Start()
	// TODO: replace the below with more suitable queries (e.g., version, node status, etc..)
	params, err := babylonClient.QueryEpochingParams()
	if err != nil {
		log.Errorf("testing babylon client: %v", err)
	} else {
		log.Infof("epoching params: %v", params)
	}

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
	utils.AddInterruptHandler(func() {
		log.Info("Stopping BTC client...")
		btcClient.Stop()
		log.Info("BTC client shutdown")
	})
	utils.AddInterruptHandler(func() {
		log.Info("Stopping Babylon client...")
		babylonClient.Stop()
		log.Info("Babylon client shutdown")
	})

	<-utils.InterruptHandlersDone
	log.Info("Shutdown complete")
}
