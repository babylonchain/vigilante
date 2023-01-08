package monitor

import (
	"fmt"
	bbnclient "github.com/babylonchain/rpc-client/client"
	"github.com/babylonchain/vigilante/btcclient"
	"github.com/babylonchain/vigilante/cmd/utils"
	"github.com/babylonchain/vigilante/config"
	vlog "github.com/babylonchain/vigilante/log"
	"github.com/babylonchain/vigilante/metrics"
	"github.com/babylonchain/vigilante/monitor"
	"github.com/babylonchain/vigilante/monitor/btcscanner"
	"github.com/babylonchain/vigilante/rpcserver"
	"github.com/babylonchain/vigilante/types"
	"github.com/spf13/cobra"
)

const (
	configFileNameFlag    = "config"
	genesisFileNameFlag   = "genesis"
	babylonRPCAddressFlag = "address"
	targetEpochFlag       = "target-epoch"

	ConfigFileNameDefault    = "monitor.yml"
	GenesisFileNameDefault   = "genesis.json"
	BabylonRPCAddressDefault = "http://localhost:26657"
)

var (
	log               = vlog.Logger.WithField("module", "cmd")
	cfgFile           string
	genesisFile       string
	babylonRPCAddress string
)

func addFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&cfgFile, configFileNameFlag, ConfigFileNameDefault, "config file")
	cmd.Flags().StringVar(&genesisFile, genesisFileNameFlag, GenesisFileNameDefault, "genesis file")
	cmd.Flags().StringVar(&babylonRPCAddress, babylonRPCAddressFlag, BabylonRPCAddressDefault, "babylon PRC address")
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
		err             error
		cfg             config.Config
		btcClient       *btcclient.Client
		babylonClient   *bbnclient.Client
		vigilantMonitor *monitor.Monitor
		server          *rpcserver.Server
	)

	// get the config from the given file or the default file
	cfg, err = config.New(cfgFile)
	if err != nil {
		panic(err)
	}

	// create BTC client and connect to BTC server
	// Note that monitor needs to subscribe to new BTC blocks
	btcClient, err = btcclient.NewWithBlockSubscriber(&cfg.BTC, cfg.Common.RetrySleepTime, cfg.Common.MaxRetrySleepTime)
	if err != nil {
		panic(err)
	}
	// create Babylon client. Note that requests from Babylon client are ad hoc
	babylonClient, err = bbnclient.New(&cfg.Babylon, cfg.Common.RetrySleepTime, cfg.Common.MaxRetrySleepTime)
	if err != nil {
		panic(err)
	}

	genesisInfo, err := types.GetGenesisInfoFromFile(genesisFile)
	if err != nil {
		panic(fmt.Errorf("failed to read genesis file: %w", err))
	}
	btcScanner, err := btcscanner.New(
		&cfg.BTC,
		btcClient,
		genesisInfo.GetBaseBTCHeight(),
		babylonClient.GetTagIdx(),
		cfg.Monitor.CheckpointBuffer,
	)
	// create monitor
	vigilantMonitor, err = monitor.New(&cfg.Monitor, genesisInfo, btcScanner, babylonClient)
	if err != nil {
		panic(err)
	}
	// create RPC server
	server, err = rpcserver.New(&cfg.GRPC, nil, nil, vigilantMonitor)
	if err != nil {
		panic(err)
	}

	vigilantMonitor.Start()

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
		log.Info("Stopping monitor...")
		vigilantMonitor.Stop()
		log.Info("Monitor shutdown")
	})

	<-utils.InterruptHandlersDone
	log.Info("Shutdown complete")
}
