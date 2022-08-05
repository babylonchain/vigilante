package submitter

import (
	"fmt"

	"github.com/babylonchain/vigilante/cmd/utils"
	"github.com/babylonchain/vigilante/config"
	"github.com/babylonchain/vigilante/netparams"
	"github.com/babylonchain/vigilante/rpcserver"
	"github.com/babylonchain/vigilante/vigilante"
	"github.com/spf13/cobra"
)

// GetCmd returns the cli query commands for this module
func GetCmd() *cobra.Command {
	// Group epoching queries under a subcommand
	cmd := &cobra.Command{
		Use:   "submitter",
		Short: "Vigilant submitter",
		Run: func(cmd *cobra.Command, args []string) {
			// load config
			// TODO: read config from a file
			cfg := config.DefaultConfig()
			btcParams := netparams.GetParams(cfg.BTC.NetParams)

			// create BTC client
			btcClient, err := utils.NewBTCClient(cfg)
			if err != nil {
				panic(err)
			}
			// create RPC client
			submitter, err := vigilante.NewSubmitter(btcClient, &btcParams)
			if err != nil {
				panic(err)
			}

			// keep trying BTC client
			utils.BTCClientConnectLoop(cfg, btcClient)
			// start submitter and sync
			submitter.Start()
			submitter.SynchronizeRPC(btcClient)
			// start RPC server
			server, err := rpcserver.New()
			if err != nil {
				panic(err)
			}

			// SIGINT handling stuff
			utils.AddInterruptHandler(func() {
				// TODO: Does this need to wait for the grpc server to finish up any requests?
				fmt.Println("Stopping RPC server...")
				server.Stop()
				fmt.Println("RPC server shutdown")
			})
			utils.AddInterruptHandler(func() {
				fmt.Println("Stopping BTC client...")
				if !btcClient.Disconnected() {
					btcClient.Stop()
				}
				fmt.Println("BTC client shutdown")
			})

			<-utils.InterruptHandlersDone
			fmt.Println("Shutdown complete")
		},
	}

	return cmd
}
