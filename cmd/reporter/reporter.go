package reporter

import (
	"github.com/babylonchain/vigilante/btcclient"
	"github.com/babylonchain/vigilante/cmd/utils"
	"github.com/babylonchain/vigilante/config"
	vlog "github.com/babylonchain/vigilante/log"
	"github.com/babylonchain/vigilante/netparams"
	"github.com/babylonchain/vigilante/rpcserver"
	"github.com/babylonchain/vigilante/vigilante"
	"github.com/spf13/cobra"
)

var (
	cfgFile string
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
	btcParams := netparams.GetParams(cfg.BTC.NetParams)

	// create BTC client
	btcClient, err := btcclient.New(&cfg.BTC)
	if err != nil {
		panic(err)
	}
	// create RPC client
	reporter, err := vigilante.NewReporter(&cfg.Reporter, btcClient, &btcParams)
	if err != nil {
		panic(err)
	}

	// keep trying BTC client
	btcClient.ConnectLoop(&cfg.BTC)
	// start reporter and sync
	reporter.Start()
	reporter.SynchronizeRPC(btcClient)
	// start RPC server
	server, err := rpcserver.New(&cfg.GRPC)
	if err != nil {
		panic(err)
	}

	// SIGINT handling stuff
	utils.AddInterruptHandler(func() {
		// TODO: Does this need to wait for the grpc server to finish up any requests?
		vlog.Logger.WithField("module", "cmd").Info("Stopping RPC server...")
		server.Stop()
		vlog.Logger.WithField("module", "cmd").Info("RPC server shutdown")
	})
	utils.AddInterruptHandler(func() {
		vlog.Logger.WithField("module", "cmd").Info("Stopping BTC client...")
		btcClient.Stop()
		vlog.Logger.WithField("module", "cmd").Info("BTC client shutdown")
	})

	<-utils.InterruptHandlersDone
	vlog.Logger.WithField("module", "cmd").Info("Shutdown complete")
}
