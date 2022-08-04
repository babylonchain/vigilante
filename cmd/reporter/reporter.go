package reporter

import (
	"github.com/babylonchain/vigilante/btcclient"
	"github.com/babylonchain/vigilante/netparams"
	"github.com/babylonchain/vigilante/rpcserver"
	"github.com/babylonchain/vigilante/vigilante"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/spf13/cobra"
)

// GetCmd returns the cli query commands for this module
func GetCmd() *cobra.Command {
	// Group epoching queries under a subcommand
	cmd := &cobra.Command{
		Use:   "reporter",
		Short: "Vigilant reporter",
		Run: func(cmd *cobra.Command, args []string) {
			// creat stuff
			// TODO: specify btc params in cmd / config file
			btcParams := netparams.SimNetParams
			// TODO: specify btc params in cmd / config file
			btcClient, err := btcclient.New(&chaincfg.SimNetParams, "localhost:18554", "user", "pass", []byte{}, false, 5)
			if err != nil {
				panic(err)
			}
			reporter := vigilante.NewReporter(btcClient, &btcParams)

			// start stuff
			if err := btcClient.Start(); err != nil {
				panic(err)
			}
			reporter.Start()
			if _, err = rpcserver.New(); err != nil {
				panic(err)
			}

			// TODO: SIGINT handling stuff
		},
	}

	return cmd
}
