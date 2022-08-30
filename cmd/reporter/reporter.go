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
	log           = vlog.Logger.WithField("module", "cmd")
	cfgFile       = ""
	babylonKeyDir = ""
)

func addFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&cfgFile, "config", "", "config file")
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
	// get the config from either
	// - a certain file specified in CLI
	// - the default file, or
	// - the default hardcoded one

	var (
		err              error
		cfg              config.Config
		cache            *btcclient.BTCCache
		btcClient        *btcclient.Client
		babylonClient    *babylonclient.Client
		vigilantReporter *reporter.Reporter
		server           *rpcserver.Server
	)

	if len(cfgFile) != 0 {
		cfg, err = config.NewFromFile(cfgFile)
	} else {
		cfg, err = config.New()
	}
	if err != nil {
		panic(err)
	}

	// apply the flags from CLI
	if len(babylonKeyDir) != 0 {
		cfg.Babylon.KeyDirectory = babylonKeyDir
	}

	// create BTC client and connect to BTC server
	btcClient, err = btcclient.New(&cfg.BTC)
	if err != nil {
		panic(err)
	}

	// create Cache to bootstrap BTC blocks
	//TODO: configure maxEntries for cache
	cache = btcclient.NewBTCCache(10)

	err = cache.Init(btcClient.Client)
	if err != nil {
		panic(err)
	}

	// create Babylon client. Note that requests from Babylon client are ad hoc
	babylonClient, err = babylonclient.New(&cfg.Babylon)
	if err != nil {
		panic(err)
	}
	// create reporter
	vigilantReporter, err = reporter.New(&cfg.Reporter, btcClient, babylonClient)
	if err != nil {
		panic(err)
	}
	// create RPC server
	server, err = rpcserver.New(&cfg.GRPC, nil, vigilantReporter)
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

	<-utils.InterruptHandlersDone
	log.Info("Shutdown complete")
}
