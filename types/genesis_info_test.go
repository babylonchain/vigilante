package types

import (
	"context"
	"fmt"
	"github.com/babylonchain/babylon/testutil/datagen"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"math/rand"
	"testing"

	"github.com/babylonchain/babylon/app"
	bbncmd "github.com/babylonchain/babylon/cmd/babylond/cmd"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/server"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	genutiltest "github.com/cosmos/cosmos-sdk/x/genutil/client/testutil"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	tmlog "github.com/tendermint/tendermint/libs/log"
)

func FuzzGetGenesisInfoFromFile(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		rand.Seed(seed)
		home := t.TempDir()
		encodingConfig := app.MakeTestEncodingConfig()
		logger := tmlog.NewNopLogger()
		cfg, err := genutiltest.CreateDefaultTendermintConfig(home)
		require.NoError(t, err)

		err = genutiltest.ExecInitCmd(app.ModuleBasics, home, encodingConfig.Marshaler)
		require.NoError(t, err)

		serverCtx := server.NewContext(viper.New(), cfg, logger)
		clientCtx := client.Context{}.
			WithCodec(encodingConfig.Marshaler).
			WithHomeDir(home).
			WithTxConfig(encodingConfig.TxConfig)

		ctx := context.Background()
		ctx = context.WithValue(ctx, server.ServerContextKey, serverCtx)
		ctx = context.WithValue(ctx, client.ClientContextKey, &clientCtx)
		cmd := bbncmd.TestnetCmd(app.ModuleBasics, banktypes.GenesisBalancesIterator{})

		validatorNum := rand.Intn(10) + 1
		epochInterval := rand.Intn(500) + 2
		baseHeight := rand.Intn(2000) + 1
		cmd.SetArgs([]string{
			fmt.Sprintf("--%s=test", flags.FlagKeyringBackend),
			fmt.Sprintf("--v=%v", validatorNum),
			fmt.Sprintf("--output-dir=%s", home),
			fmt.Sprintf("--btc-base-header-height=%v", baseHeight),
			fmt.Sprintf("--epoch-interval=%v", epochInterval),
		})
		err = cmd.ExecuteContext(ctx)
		require.NoError(t, err)

		genFile := cfg.GenesisFile()
		genesisInfo, err := GetGenesisInfoFromFile(genFile)
		require.NoError(t, err)
		require.Equal(t, uint64(epochInterval), genesisInfo.epochInterval)
		require.Len(t, genesisInfo.valSet.ValSet, validatorNum)
		require.Equal(t, uint64(baseHeight), genesisInfo.baseBTCHeight)
	})
}
