package babylonclient

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (c *Client) GetAddr() (sdk.AccAddress, error) {
	return c.ChainClient.GetKeyAddress()
}

func (c *Client) MustGetAddr() sdk.AccAddress {
	addr, err := c.ChainClient.GetKeyAddress()
	if err != nil {
		panic(fmt.Errorf("Failed to get signer: %v", err))
	}
	return addr
}
