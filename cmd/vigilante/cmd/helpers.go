package cmd

import (
	"encoding/json"
	"fmt"
	"strconv"

	"cosmossdk.io/math"
	"github.com/babylonchain/vigilante/config"
	"github.com/babylonchain/vigilante/types"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/spf13/cobra"

	babylontypes "github.com/babylonchain/babylon/types"
	btcltypes "github.com/babylonchain/babylon/x/btclightclient/types"
)

// Helpers helpers for manipulating data.
func Helpers() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        "helpers",
		Short:                      "Useful commands for manipulating data",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}
	cmd.AddCommand(cmdHelperBtcHeader())

	return cmd
}

func cmdHelperBtcHeader() *cobra.Command {
	var cfgFile = ""

	cmd := &cobra.Command{
		Use:     "btc-base-header [btc-heigth]",
		Example: "btc-base-header",
		Short:   "Returns the babylon BTCHeaderInfo from btc block",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			blkHeight, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("failed to parse blk height: %w", err)
			}

			// get the config from the given file or the default file
			cfg, err := config.New(cfgFile)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// TODO Currently we are not using Params field of rpcclient.ConnConfig due to bug in btcd
			// when handling signet.
			connCfg := &rpcclient.ConnConfig{
				Host:         cfg.BTC.WalletEndpoint,
				HTTPPostMode: true,
				User:         cfg.BTC.Username,
				Pass:         cfg.BTC.Password,
				DisableTLS:   cfg.BTC.DisableClientTLS,
				Certificates: cfg.BTC.ReadWalletCAFile(),
				Params:       cfg.BTC.NetParams,
				Endpoint:     "ws",
			}

			rpcClient, err := rpcclient.New(connCfg, nil)
			if err != nil {
				return err
			}

			blkHash, err := rpcClient.GetBlockHash(int64(blkHeight))
			if err != nil {
				return err
			}

			mBlock, err := rpcClient.GetBlock(blkHash)
			if err != nil {
				return fmt.Errorf("failed to get block by hash %s: %w", blkHash.String(), err)
			}

			idxBlocks := types.NewIndexedBlockFromMsgBlock(int32(blkHeight), mBlock)
			btcInfoHeader := babylontypes.NewBTCHeaderBytesFromBlockHeader(idxBlocks.Header)
			headerHash := babylontypes.NewBTCHeaderHashBytesFromChainhash(blkHash)
			work := math.NewUint(2)

			headerInfo := btcltypes.NewBTCHeaderInfo(&btcInfoHeader, &headerHash, uint64(blkHeight), &work)
			jsonBz, err := json.MarshalIndent(headerInfo, "", "    ")
			if err != nil {
				return fmt.Errorf("failed to marshal blk header %+v: %w", headerInfo, err)
			}
			fmt.Println(string(jsonBz))
			return nil
		},
	}

	cmd.Flags().StringVar(&cfgFile, "config", config.DefaultConfigFile(), "config file")
	return cmd
}
