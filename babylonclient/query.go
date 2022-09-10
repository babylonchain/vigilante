package babylonclient

import (
	bbntypes "github.com/babylonchain/babylon/types"
	btcctypes "github.com/babylonchain/babylon/x/btccheckpoint/types"
	btclctypes "github.com/babylonchain/babylon/x/btclightclient/types"
	checkpointingtypes "github.com/babylonchain/babylon/x/checkpointing/types"
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

// QueryBTCLightclientParams queries btclightclient module's parameters via ChainClient
func (c *Client) QueryBTCLightclientParams() (*btclctypes.Params, error) {
	query := query.Query{Client: c.ChainClient, Options: query.DefaultOptions()}
	ctx, cancel := query.GetQueryContext()
	defer cancel()

	queryClient := btclctypes.NewQueryClient(c.ChainClient)
	req := &btclctypes.QueryParamsRequest{}
	resp, err := queryClient.Params(ctx, req)
	if err != nil {
		return &btclctypes.Params{}, err
	}
	return &resp.Params, nil
}

// QueryBTCCheckpointParams queries btccheckpoint module's parameters via ChainClient
func (c *Client) QueryBTCCheckpointParams() (*btcctypes.Params, error) {
	query := query.Query{Client: c.ChainClient, Options: query.DefaultOptions()}
	ctx, cancel := query.GetQueryContext()
	defer cancel()

	queryClient := btcctypes.NewQueryClient(c.ChainClient)
	req := &btcctypes.QueryParamsRequest{}
	resp, err := queryClient.Params(ctx, req)
	if err != nil {
		return &btcctypes.Params{}, err
	}
	return &resp.Params, nil
}

// QueryHeaderChainTip queries hash/height of the latest BTC block in the btclightclient module
func (c *Client) QueryHeaderChainTip() (*chainhash.Hash, uint64, error) {
	query := query.Query{Client: c.ChainClient, Options: query.DefaultOptions()}
	ctx, cancel := query.GetQueryContext()
	defer cancel()

	queryClient := btclctypes.NewQueryClient(c.ChainClient)
	req := &btclctypes.QueryTipRequest{}
	resp, err := queryClient.Tip(ctx, req)
	if err != nil {
		return nil, 0, err
	}

	return resp.Header.Hash.ToChainhash(), resp.Header.Height, nil
}

func (c *Client) QueryRawCheckpoint(epochNumber uint64) (*checkpointingtypes.RawCheckpointWithMeta, error) {
	query := query.Query{Client: c.ChainClient, Options: query.DefaultOptions()}
	ctx, cancel := query.GetQueryContext()
	defer cancel()

	queryClient := checkpointingtypes.NewQueryClient(c.ChainClient)
	req := &checkpointingtypes.QueryRawCheckpointRequest{
		EpochNum: epochNumber,
	}
	resp, err := queryClient.RawCheckpoint(ctx, req)
	if err != nil {
		return &checkpointingtypes.RawCheckpointWithMeta{}, err
	}
	return resp.RawCheckpoint, nil
}

func (c *Client) QueryRawCheckpointList(status checkpointingtypes.CheckpointStatus) ([]*checkpointingtypes.RawCheckpointWithMeta, error) {
	query := query.Query{Client: c.ChainClient, Options: query.DefaultOptions()}
	ctx, cancel := query.GetQueryContext()
	defer cancel()

	queryClient := checkpointingtypes.NewQueryClient(c.ChainClient)
	req := &checkpointingtypes.QueryRawCheckpointListRequest{
		Status:     status,
		Pagination: query.Options.Pagination, // TODO: parameterise/customise pagination options
	}
	resp, err := queryClient.RawCheckpointList(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.RawCheckpoints, nil
}

func (c *Client) QueryContainsBlock(blockHash *chainhash.Hash) (bool, error) {
	query := query.Query{Client: c.ChainClient, Options: query.DefaultOptions()}
	ctx, cancel := query.GetQueryContext()
	defer cancel()

	queryClient := btclctypes.NewQueryClient(c.ChainClient)
	btcHeaderHashBytes := bbntypes.NewBTCHeaderHashBytesFromChainhash(blockHash)
	req := &btclctypes.QueryContainsRequest{Hash: &btcHeaderHashBytes}
	resp, err := queryClient.Contains(ctx, req)
	if err != nil {
		return false, err
	}

	return resp.Contains, nil
}

// TODO: implement necessary queries here
// TODO: simplify query constructions via Go generics
