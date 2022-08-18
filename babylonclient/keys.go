package babylonclient

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (c *Client) GetAddr() (sdk.AccAddress, error) {
	keys, err := c.Keybase.List()
	if err != nil {
		return nil, err
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("no key pair found in keyring")
	}
	// TODO: what if there are many keys?
	return keys[0].GetAddress(), nil
}
