package types

import (
	"fmt"

	babylontypes "github.com/babylonchain/babylon/types"
	btcltypes "github.com/babylonchain/babylon/x/btclightclient/types"
	"github.com/btcsuite/btcd/wire"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func NewMsgInsertHeader(prefix string, signer sdk.AccAddress, header *wire.BlockHeader) *btcltypes.MsgInsertHeader {
	signerBech32 := sdk.MustBech32ifyAddressBytes(prefix, signer)
	headerBytes := babylontypes.NewBTCHeaderBytesFromBlockHeader(header)
	return &btcltypes.MsgInsertHeader{
		Signer: signerBech32, // TODO: avoid using accAddress.String() everywhere (including babylonchain/babylon) since it uses the default prefix of Cosmos SDK
		Header: &headerBytes,
	}
}

// PaginateHeaderMsgs split a given list of MsgInsertHeader msgs into pages with a given page size
func PaginateHeaderMsgs(slice []*btcltypes.MsgInsertHeader, pageSize int) ([][]*btcltypes.MsgInsertHeader, error) {
	// page size has to be positive
	if pageSize <= 0 {
		return nil, fmt.Errorf("pageSize has to be positive")
	}

	var pages [][]*btcltypes.MsgInsertHeader

	// Calculate the number of pages needed to hold all elements in the slice.
	numPages := (len(slice) + pageSize - 1) / pageSize

	// Iterate through the slice and split it into pages.
	for i := 0; i < numPages; i++ {
		startIndex := i * pageSize
		endIndex := (i + 1) * pageSize

		// Ensure endIndex does not go beyond the length of the slice.
		if endIndex > len(slice) {
			endIndex = len(slice)
		}

		page := slice[startIndex:endIndex]
		pages = append(pages, page)
	}

	return pages, nil
}
