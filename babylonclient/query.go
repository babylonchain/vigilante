package babylonclient

import (
	btclightclienttypes "github.com/babylonchain/babylon/x/btclightclient/types"
	epochingtypes "github.com/babylonchain/babylon/x/epoching/types"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/strangelove-ventures/lens/client/query"
)

// QueryStakingParams queries staking module's parameters via ChainClient
// code is adapted from https://github.com/strangelove-ventures/lens/blob/v0.5.1/cmd/staking.go#L128-L149
func (c *Client) QueryStakingParams() (*stakingtypes.Params, error) {
	query := query.Query{Client: c.ChainClient, Options: query.DefaultOptions()} // TODO: what's the impact of DefaultOptions()?
	resp, err := query.Staking_Params()
	if err != nil {
		return &stakingtypes.Params{}, err
	}

	return &resp.Params, nil
}

// QueryEpochingParams queries epoching module's parameters via ChainClient
// code is adapted from https://github.com/strangelove-ventures/lens/blob/v0.5.1/client/query/staking.go#L7-L18
func (c *Client) QueryEpochingParams() (*epochingtypes.Params, error) {
	query := query.Query{Client: c.ChainClient, Options: query.DefaultOptions()}
	ctx, cancel := query.GetQueryContext()
	defer cancel()

	queryClient := epochingtypes.NewQueryClient(c.ChainClient)
	req := &epochingtypes.QueryParamsRequest{}
	resp, err := queryClient.Params(ctx, req)
	if err != nil {
		return &epochingtypes.Params{}, err
	}
	return &resp.Params, nil
}

// QueryHeaderChainTip queries hash/height of the latest BTC block in the btclightclient module
func (c *Client) QueryHeaderChainTip() (*chainhash.Hash, uint64, error) {
	query := query.Query{Client: c.ChainClient, Options: query.DefaultOptions()}
	ctx, cancel := query.GetQueryContext()
	defer cancel()

	queryClient := btclightclienttypes.NewQueryClient(c.ChainClient)
	req := &btclightclienttypes.QueryTipRequest{}
	resp, err := queryClient.Tip(ctx, req)
	if err != nil {
		return nil, 0, err
	}

	return resp.Header.Hash.ToChainhash(), resp.Header.Height, nil
}

// TODO: implement necessary queries here
