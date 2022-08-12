package bblclient

import (
	epochingtypes "github.com/babylonchain/babylon/x/epoching/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/strangelove-ventures/lens/client/query"
)

// QueryStakingParams queries staking module's parameters via ChainClient
// code is adapted from https://github.com/strangelove-ventures/lens/blob/main/cmd/staking.go#L128-L149
func (c *Client) QueryStakingParams() (stakingtypes.Params, error) {
	query := query.Query{Client: c.ChainClient, Options: query.DefaultOptions()} // TODO: what's the impact of DefaultOptions()?
	resp, err := query.Staking_Params()
	if err != nil {
		return stakingtypes.Params{}, err
	}

	return resp.Params, nil
}

// QueryEpochingParams queries epoching module's parameters via ChainClient
// code is adapted from https://github.com/strangelove-ventures/lens/blob/main/client/query/staking.go#L7-L18
func (c *Client) QueryEpochingParams() (epochingtypes.Params, error) {
	query := query.Query{Client: c.ChainClient, Options: query.DefaultOptions()} // TODO: what's the impact of DefaultOptions()?
	ctx, cancel := query.GetQueryContext()
	defer cancel()

	queryClient := epochingtypes.NewQueryClient(c.ChainClient)
	req := &epochingtypes.QueryParamsRequest{}
	resp, err := queryClient.Params(ctx, req)
	if err != nil {
		return epochingtypes.Params{}, err
	}
	return resp.Params, nil
}

// TODO: implement necessary queries here
