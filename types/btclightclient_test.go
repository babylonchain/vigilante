package types_test

import (
	"math/rand"
	"testing"

	"github.com/babylonchain/babylon/testutil/datagen"
	btclctypes "github.com/babylonchain/babylon/x/btclightclient/types"
	"github.com/babylonchain/vigilante/types"
	"github.com/stretchr/testify/require"
)

func FuzzPaginateHeaderMsgs(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		// random page size
		pageSize := datagen.RandomInt(r, 10) + 1
		// random number of headers (at least 1 page)
		numHeaders := pageSize + datagen.RandomInt(r, 100)

		// generate a list of BTC headers
		prefix := "bbn"
		signer := datagen.GenRandomAccount().GetAddress()
		msgs := []*btclctypes.MsgInsertHeader{}
		for i := uint64(0); i < numHeaders; i++ {
			header := datagen.GenRandomBtcdHeader(r)
			msg := types.NewMsgInsertHeader(prefix, signer, header)
			msgs = append(msgs, msg)
		}

		// paginate
		pages, err := types.PaginateHeaderMsgs(msgs, int(pageSize))
		require.NoError(t, err)

		// assert pages has same number of headers as in msgs
		actualNumHeaders := 0
		for _, page := range pages {
			actualNumHeaders += len(page)
		}
		require.Equal(t, len(msgs), actualNumHeaders)

		// assert equivalence of headers
		for i, page := range pages {
			for j, msg := range page {
				idx := i*int(pageSize) + j
				require.Equal(t, msgs[idx].Header.MustMarshal(), msg.Header.MustMarshal())
			}
		}
	})
}
