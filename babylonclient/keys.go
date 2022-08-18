package babylonclient

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (c *Client) GetAddr() (sdk.AccAddress, error) {
	return c.ChainClient.GetKeyAddress()
}
