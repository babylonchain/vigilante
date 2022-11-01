package babylonclient_test

import (
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/babylonchain/babylon/testutil/datagen"
	"github.com/babylonchain/vigilante/babylonclient"
	"github.com/babylonchain/vigilante/config"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/stretchr/testify/require"
)

func FuzzKeys(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		rand.Seed(seed)

		// create a keyring
		keyringName := datagen.GenRandomHexStr(10)
		dir := t.TempDir()
		mockIn := strings.NewReader("")
		kr, err := keyring.New(keyringName, "test", dir, mockIn)
		require.NoError(t, err)

		// create a random key pair in this keyring
		keyName := datagen.GenRandomHexStr(10)
		kr.NewMnemonic(
			keyName,
			keyring.English,
			hd.CreateHDPath(118, 0, 0).String(),
			keyring.DefaultBIP39Passphrase,
			hd.Secp256k1,
		)

		// create a Babylon client with this random keyring
		cfg := config.DefaultBabylonConfig()
		cfg.KeyDirectory = dir
		cfg.Key = keyName
		cl, err := babylonclient.New(&cfg, 1*time.Minute, 5*time.Minute)
		require.NoError(t, err)

		// retrieve the key info from key ring
		keys, err := kr.List()
		require.NoError(t, err)
		require.Equal(t, 1, len(keys))

		// test if the key is consistent in Babylon client and keyring
		bbnAddr := cl.MustGetAddr()
		require.Equal(t, keys[0].GetAddress(), bbnAddr)
	})
}
