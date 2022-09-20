package types_test

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/babylonchain/babylon/btctxformatter"
	"github.com/babylonchain/vigilante/netparams"
	"github.com/babylonchain/vigilante/types"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/wire"
	"github.com/stretchr/testify/require"
)

// TODO: tests on CkptSegment and CkptSegmentPool

var (
	tx1Hex = "01000000016712063937f821c4b79d55d3cf712ed5c825dedcc52d3b5b60cfd51d730609e2000000006b483045022100df0540936ff15ac98d49c2fb0b16eb31b1458d131b5b9ed627da0da0a6b85ba2022055ec63da913de116df127f19de85cb61a371c446814960d294781e92e780076e012103a98409a780694b1d2454f2b128cf694a966dd39edaec189a6fe03cd8b2690b34ffffffff020000000000000000516a4c4e6262743000000000000000000092ee5480cf702c2fdf156788c6a6131e363e44a86f230659a3392b00330bb6bb0a00000000000000000000000066adeb1607db6365171c28f93ec20b0505debfe100f2052a010000001976a914a11bf0378519c58f1cfee223f8831a877d03147a88ac00000000"
	tx2Hex = "0100000001b58b1c1f98caa5e92afd9ace05a9cace8bd68bd7e73a795e8e9e685b496e3d68000000006a47304402201a6b4d7d0be87138e0af256239a0f966471e59ddaac711f289ef1cb5d495e59d02205151982bd60b0a402eaba55b150634f981e956d05aaa457c9904c79302c7e280012103a98409a780694b1d2454f2b128cf694a966dd39edaec189a6fe03cd8b2690b34ffffffff020000000000000000416a3f6262743010a52f5a519b2843dce9dff88124d49f8ebf080763cdfcf14c169e7c00098bef52812bfee988a49e66e51bbbbd782057eceb3b8481235ab382ae3100f2052a010000001976a914b9ad70dc3a02004314b94247819d9ad943a3a3e288ac00000000"
)

func toWrappedTx(txHex string) (*btcutil.Tx, error) {
	decodedTx, err := hex.DecodeString(txHex)
	if err != nil {
		return nil, err
	}
	tx := wire.NewMsgTx(0)
	tx.Deserialize(bytes.NewReader(decodedTx))
	wrappedTx := btcutil.NewTx(tx)
	return wrappedTx, nil
}

func TestGetBabylonDataFromTx(t *testing.T) {
	bbnParams := netparams.GetBabylonParams("simnet", 48)
	tag, version := bbnParams.Tag, bbnParams.Version

	wTx1, err := toWrappedTx(tx1Hex)
	require.NoError(t, err)
	wTx2, err := toWrappedTx(tx2Hex)
	require.NoError(t, err)

	seg1 := types.GetBabylonDataFromTx(tag, version, wTx1)
	require.NotNil(t, seg1)
	seg2 := types.GetBabylonDataFromTx(tag, version, wTx2)
	require.NotNil(t, seg2)

	ckpt, err := btctxformatter.ConnectParts(version, seg1.Data, seg2.Data)
	require.NoError(t, err)
	require.NotEmpty(t, ckpt)
}
