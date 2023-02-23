package monitor

import (
	"fmt"

	bbnqccfg "github.com/babylonchain/rpc-client/config"
	bbnqc "github.com/babylonchain/rpc-client/query"
	"github.com/spf13/cobra"

	"github.com/babylonchain/vigilante/btcclient"
	"github.com/babylonchain/vigilante/cmd/utils"
	"github.com/babylonchain/vigilante/config"
	vlog "github.com/babylonchain/vigilante/log"
	"github.com/babylonchain/vigilante/metrics"
	"github.com/babylonchain/vigilante/monitor"
	"github.com/babylonchain/vigilante/monitor/btcscanner"
	"github.com/babylonchain/vigilante/rpcserver"
	"github.com/babylonchain/vigilante/types"
)

const (
	configFileNameFlag  = "config"
	genesisFileNameFlag = "genesis"

	ConfigFileNameDefault  = "vigilante.yml"
	GenesisFileNameDefault = "genesis.json"
)

var (
	log         = vlog.Logger.WithField("module", "cmd")
	cfgFile     string
	genesisFile string
)

func addFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&cfgFile, configFileNameFlag, ConfigFileNameDefault, "config file")
	cmd.Flags().StringVar(&genesisFile, genesisFileNameFlag, GenesisFileNameDefault, "genesis file")
}

// GetCmd returns the cli query commands for this module
func GetCmd() *cobra.Command {
	// Group epoching queries under a subcommand
	cmd := &cobra.Command{
		Use:   "monitor",
		Short: "Vigilante monitor constantly checks the consistency between the Babylon node and BTC and detects censorship of BTC checkpoints",
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
		bbnQueryClient   bbnqc.BabylonQueryClient
		vigilanteMonitor *monitor.Monitor
		server           *rpcserver.Server
	)

	// get the config from the given file or the default file
	cfg, err = config.New(cfgFile)
	if err != nil {
		panic(fmt.Errorf("failed to load config: %w", err))
	}

	// create BTC client and connect to BTC server
	// Note that monitor needs to subscribe to new BTC blocks
	btcClient, err = btcclient.NewWithBlockSubscriber(&cfg.BTC, cfg.Common.RetrySleepTime, cfg.Common.MaxRetrySleepTime)
	if err != nil {
		panic(fmt.Errorf("failed to open BTC client: %w", err))
	}

	// create Babylon query client. Note that requests from Babylon client are ad hoc
	queryCfg := &bbnqccfg.BabylonQueryConfig{
		RPCAddr: cfg.Babylon.RPCAddr,
		Timeout: cfg.Babylon.Timeout,
	}
	err = queryCfg.Validate()
	if err != nil {
		panic(fmt.Errorf("invalid config for query client: %w", err))
	}
	bbnQueryClient, err = bbnqc.New(queryCfg)
	if err != nil {
		panic(fmt.Errorf("failed to create babylon query client: %w", err))
	}

	genesisInfo, err := types.GetGenesisInfoFromFile(genesisFile)
	if err != nil {
		panic(fmt.Errorf("failed to read genesis file: %w", err))
	}
	btcScanner, err := btcscanner.New(
		&cfg.BTC,
		&cfg.Monitor,
		btcClient,
		genesisInfo.GetBaseBTCHeight(),
		cfg.Babylon.TagIdx,
	)
	if err != nil {
		panic(fmt.Errorf("failed to create BTC scanner: %w", err))
	}
	// create monitor
	vigilanteMonitor, err = monitor.New(&cfg.Monitor, genesisInfo, btcScanner, bbnQueryClient)
	if err != nil {
		panic(fmt.Errorf("failed to create vigilante monitor: %w", err))
	}
	// create RPC server
	server, err = rpcserver.New(&cfg.GRPC, nil, nil, vigilanteMonitor)
	if err != nil {
		panic(fmt.Errorf("failed to create monitor's RPC server: %w", err))
	}

	vigilanteMonitor.Start()

	// start RPC server
	server.Start()
	// start Prometheus metrics server
	addr := fmt.Sprintf("%s:%d", cfg.Metrics.Host, cfg.Metrics.ServerPort)
	metrics.Start(addr)

	// SIGINT handling stuff
	utils.AddInterruptHandler(func() {
		// TODO: Does this need to wait for the grpc server to finish up any requests?
		log.Info("Stopping RPC server...")
		server.Stop()
		log.Info("RPC server shutdown")
	})
	utils.AddInterruptHandler(func() {
		log.Info("Stopping monitor...")
		vigilanteMonitor.Stop()
		log.Info("Monitor shutdown")
	})

	<-utils.InterruptHandlersDone
	log.Info("Shutdown complete")
}
