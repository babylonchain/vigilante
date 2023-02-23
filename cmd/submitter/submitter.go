package submitter

import (
	"fmt"

	bbnqccfg "github.com/babylonchain/rpc-client/config"
	bbnqc "github.com/babylonchain/rpc-client/query"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/spf13/cobra"

	"github.com/babylonchain/vigilante/btcclient"
	"github.com/babylonchain/vigilante/cmd/utils"
	"github.com/babylonchain/vigilante/config"
	vlog "github.com/babylonchain/vigilante/log"
	"github.com/babylonchain/vigilante/metrics"
	"github.com/babylonchain/vigilante/rpcserver"
	"github.com/babylonchain/vigilante/submitter"
)

var (
	cfgFile = ""
	log     = vlog.Logger.WithField("module", "cmd")
)

// GetCmd returns the cli query commands for this module
func GetCmd() *cobra.Command {
	// Group epoching queries under a subcommand
	cmd := &cobra.Command{
		Use:   "submitter",
		Short: "Vigilant submitter",
		Run:   cmdFunc,
	}
	addFlags(cmd)
	return cmd
}

func addFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&cfgFile, "config", config.DefaultConfigFile(), "config file")
}

func cmdFunc(cmd *cobra.Command, args []string) {
	// get the config from the given file or the default file
	cfg, err := config.New(cfgFile)
	if err != nil {
		panic(fmt.Errorf("failed to load config: %w", err))
	}

	// create BTC wallet and connect to BTC server
	btcWallet, err := btcclient.NewWallet(&cfg.BTC)
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

	// create submitter
	vigilantSubmitter, err := submitter.New(&cfg.Submitter, btcWallet, queryClient, submitterAddr, cfg.Babylon.TagIdx)
	if err != nil {
		panic(fmt.Errorf("failed to create vigilante submitter: %w", err))
	}

	// create RPC server
	server, err := rpcserver.New(&cfg.GRPC, vigilantSubmitter, nil, nil)
	if err != nil {
		panic(fmt.Errorf("failed to create submitter's RPC server: %w", err))
	}

	// start submitter and sync
	vigilantSubmitter.Start()

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
		log.Info("Stopping submitter...")
		vigilantSubmitter.Stop()
		log.Info("Submitter shutdown")
	})

	<-utils.InterruptHandlersDone
	log.Info("Shutdown complete")
}
