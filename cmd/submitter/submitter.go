package submitter

import (
	"github.com/babylonchain/vigilante/babylonclient"
	"github.com/babylonchain/vigilante/btcclient"
	"github.com/babylonchain/vigilante/cmd/utils"
	"github.com/babylonchain/vigilante/config"
	vlog "github.com/babylonchain/vigilante/log"
	"github.com/babylonchain/vigilante/metrics"
	"github.com/babylonchain/vigilante/rpcserver"
	"github.com/babylonchain/vigilante/submitter"
	sdk "github.com/cosmos/cosmos-sdk/types"
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
		Use:   "submitter",
		Short: "Vigilant submitter",
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

	// TODO: add submitter addr
	address := sdk.AccAddress{}
	// TODO: add wallet account
	account := ""

	// create BTC client and connect to BTC server
	// Note that vigilant reporter needs to subscribe to new BTC blocks
	btcClient, err := btcclient.New(&cfg.BTC)
	if err != nil {
		panic(err)
	}
	// create Babylon client. Note that requests from Babylon client are ad hoc
	babylonClient, err := babylonclient.New(&cfg.Babylon)
	if err != nil {
		panic(err)
	}
	// create submitter
	vigilantSubmitter, err := submitter.New(&cfg.Submitter, btcClient, babylonClient, address, account)
	if err != nil {
		panic(err)
	}
	// create RPC server
	server, err := rpcserver.New(&cfg.GRPC, vigilantSubmitter, nil)
	if err != nil {
		panic(err)
	}

	// start submitter and sync
	vigilantSubmitter.Start()
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
		log.Info("Stopping submitter...")
		vigilantSubmitter.Stop()
		log.Info("Submitter shutdown")
	})

	<-utils.InterruptHandlersDone
	log.Info("Shutdown complete")
}
