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
	tx1Hex = "0100000001e41148d7079da16b7852d6eb97503c2812eb04bd5171cdfaa3fa05866992f57e000000006b483045022100af93eb1b669a0a24ac23ba8e81244ae93576ca0aaa7088c458e72358c4287dea022010f5d79c1e71705628992c4288663ac52eac1b48e7dd8c326f3d65437e60c48301210202059b159194f81b2017f98c32622850a74cd155bb49cf098dbcfcf5a8ab9b02ffffffff020000000000000000506a4c4d4242540000000000000000008eebaf2def594d8bd876c9a178769bc4f03c28178acb8412313e12c2fbd666360300000000000000000000000066adeb1607db6365171c28f93ec20b0505debfe100f2052a010000001976a91457d666b33d16262f906ce9433a8754f91949f93288ac00000000"
	tx2Hex = "0100000001f27f8f21c11059b9a295032dfadf3fb4971e7365696b3d5506f2058a4488fc89000000006b483045022100d172e9ec1d212250c2abf92323d599e66eb66597044abe3b5ae97ee04bf34b2502201dea3a4a6121c1545771fcf55bc312ec9f58a207f7fe2545ed904c1a495eaf5601210202059b159194f81b2017f98c32622850a74cd155bb49cf098dbcfcf5a8ab9b02ffffffff020000000000000000406a3e424254108dbc8d4a79214b70af1c0583cc1db0bac816f7d8e89772502b9a6bd1220756ca57810722465ee118a58ac66a0bb3877b61b540239db1551f727400f2052a010000001976a914ebb03116f87c809f9eee0cb55d388c45d9f231af88ac00000000"
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
	bbnParams := netparams.GetBabylonParams("simnet")
	tag, version := bbnParams.Tag, bbnParams.Version

	wTx1, err := toWrappedTx(tx1Hex)
	require.NoError(t, err)
	wTx2, err := toWrappedTx(tx2Hex)
	require.NoError(t, err)

	seg1 := types.GetBabylonDataFromTx(tag, version, wTx1)
	require.NotNil(t, seg1)
	seg2 := types.GetBabylonDataFromTx(tag, version, wTx2)
	require.NotNil(t, seg2)

	ckpt, err := btctxformatter.ComposeRawCheckpoint(version, seg1.Data, seg2.Data)
	require.NoError(t, err)
	require.NotEmpty(t, ckpt)
}
