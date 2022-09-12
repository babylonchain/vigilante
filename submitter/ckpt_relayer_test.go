package submitter_test

import (
	"github.com/babylonchain/babylon/testutil/datagen"
	"github.com/babylonchain/babylon/x/checkpointing/types"
	"github.com/babylonchain/vigilante/babylonclient"
	"github.com/babylonchain/vigilante/btcclient"
	"github.com/babylonchain/vigilante/config"
	"github.com/babylonchain/vigilante/submitter"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSubmitter_SubmitCkpt(t *testing.T) {
	cfg, err := config.New()
	require.NoError(t, err)
	// TODO: add submitter addr
	address := sdk.AccAddress{}
	// TODO: add wallet account
	account := ""

	btcClient, err := btcclient.New(&cfg.BTC)
	require.NoError(t, err)
	babylonClient, err := babylonclient.New(&cfg.Babylon)
	require.NoError(t, err)
	s, err := submitter.New(&cfg.Submitter, btcClient, babylonClient, address, account)
	require.NoError(t, err)

	ckptWithMeta := datagen.GenRandomRawCheckpointWithMeta()
	ckptWithMeta.Status = types.Sealed
	err = s.SubmitCkpt(ckptWithMeta)
	require.NoError(t, err)

}
