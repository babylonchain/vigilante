package types

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/babylonchain/babylon/app"
	btccheckpointtypes "github.com/babylonchain/babylon/x/btccheckpoint/types"
	btclightclienttypes "github.com/babylonchain/babylon/x/btclightclient/types"
	checkpointingtypes "github.com/babylonchain/babylon/x/checkpointing/types"
	epochingtypes "github.com/babylonchain/babylon/x/epoching/types"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

type GenesisInfo struct {
	baseBTCHeight uint64
	epochInterval uint64
	checkpointTag string
	valSet        checkpointingtypes.ValidatorWithBlsKeySet
}

func NewGenesisInfo(
	baseBTCHeight uint64,
	epochInterval uint64,
	checkpointTag string,
	valSet *checkpointingtypes.ValidatorWithBlsKeySet,
) *GenesisInfo {
	return &GenesisInfo{
		baseBTCHeight: baseBTCHeight,
		epochInterval: epochInterval,
		checkpointTag: checkpointTag,
		valSet:        *valSet,
	}
}

// GetGenesisInfoFromFile reads genesis info from the provided genesis file
func GetGenesisInfoFromFile(filePath string) (*GenesisInfo, error) {
	var (
		baseBTCHeight uint64
		epochInterval uint64
		checkpointTag string
		valSet        checkpointingtypes.ValidatorWithBlsKeySet
		err           error
	)

	appState, _, err := genutiltypes.GenesisStateFromGenFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read genesis file %v, %w", filePath, err)
	}

	tmpBabylon := app.NewTmpBabylonApp()
	gentxModule := tmpBabylon.BasicModuleManager[genutiltypes.ModuleName].(genutil.AppModuleBasic)

	checkpointingGenState := checkpointingtypes.GetGenesisStateFromAppState(tmpBabylon.AppCodec(), appState)
	err = checkpointingGenState.Validate()
	if err != nil {
		return nil, fmt.Errorf("invalid checkpointing genesis %w", err)
	}
	genutilGenState := genutiltypes.GetGenesisStateFromAppState(tmpBabylon.AppCodec(), appState)
	gentxs := genutilGenState.GenTxs
	gks := checkpointingGenState.GetGenesisKeys()

	valSet.ValSet = make([]*checkpointingtypes.ValidatorWithBlsKey, 0)
	for _, gk := range gks {
		for _, tx := range gentxs {
			tx, err := genutiltypes.ValidateAndGetGenTx(tx, tmpBabylon.TxConfig().TxJSONDecoder(), gentxModule.GenTxValidator)
			if err != nil {
				return nil, fmt.Errorf("invalid genesis tx %w", err)
			}
			msgs := tx.GetMsgs()
			if len(msgs) == 0 {
				return nil, errors.New("invalid genesis transaction")
			}
			msgCreateValidator := msgs[0].(*stakingtypes.MsgCreateValidator)

			if gk.ValidatorAddress == msgCreateValidator.ValidatorAddress {
				keyWithPower := &checkpointingtypes.ValidatorWithBlsKey{
					ValidatorAddress: msgCreateValidator.ValidatorAddress,
					BlsPubKey:        *gk.BlsKey.Pubkey,
					VotingPower:      uint64(sdk.TokensToConsensusPower(msgCreateValidator.Value.Amount, sdk.DefaultPowerReduction)),
				}
				valSet.ValSet = append(valSet.ValSet, keyWithPower)
			}
		}
	}

	btclightclientGenState := GetBtclightclientGenesisStateFromAppState(tmpBabylon.AppCodec(), appState)
	err = btclightclientGenState.Validate()
	if err != nil {
		return nil, fmt.Errorf("invalid btclightclient genesis %w", err)
	}
	baseBTCHeight = btclightclientGenState.BaseBtcHeader.Height

	epochingGenState := GetEpochingGenesisStateFromAppState(tmpBabylon.AppCodec(), appState)
	err = epochingGenState.Validate()
	if err != nil {
		return nil, fmt.Errorf("invalid epoching genesis %w", err)
	}
	epochInterval = epochingGenState.Params.EpochInterval

	btccheckpointGenState := GetBtccheckpointGenesisStateFromAppState(tmpBabylon.AppCodec(), appState)
	err = btccheckpointGenState.Validate()
	if err != nil {
		return nil, fmt.Errorf("invalid btccheckpoint genesis %w", err)
	}
	checkpointTag = btccheckpointGenState.Params.CheckpointTag

	genesisInfo := &GenesisInfo{
		baseBTCHeight: baseBTCHeight,
		epochInterval: epochInterval,
		checkpointTag: checkpointTag,
		valSet:        valSet,
	}

	return genesisInfo, nil
}

// GetBtclightclientGenesisStateFromAppState returns x/btclightclient GenesisState given raw application
// genesis state.
func GetBtclightclientGenesisStateFromAppState(cdc codec.Codec, appState map[string]json.RawMessage) btclightclienttypes.GenesisState {
	var genesisState btclightclienttypes.GenesisState

	if appState[btclightclienttypes.ModuleName] != nil {
		cdc.MustUnmarshalJSON(appState[btclightclienttypes.ModuleName], &genesisState)
	}

	return genesisState
}

// GetEpochingGenesisStateFromAppState returns x/epoching GenesisState given raw application
// genesis state.
func GetEpochingGenesisStateFromAppState(cdc codec.Codec, appState map[string]json.RawMessage) epochingtypes.GenesisState {
	var genesisState epochingtypes.GenesisState

	if appState[epochingtypes.ModuleName] != nil {
		cdc.MustUnmarshalJSON(appState[epochingtypes.ModuleName], &genesisState)
	}

	return genesisState
}

// GetBtccheckpointGenesisStateFromAppState returns x/btccheckpoint GenesisState given raw application
// genesis state.
func GetBtccheckpointGenesisStateFromAppState(cdc codec.Codec, appState map[string]json.RawMessage) btccheckpointtypes.GenesisState {
	var genesisState btccheckpointtypes.GenesisState

	if appState[btccheckpointtypes.ModuleName] != nil {
		cdc.MustUnmarshalJSON(appState[btccheckpointtypes.ModuleName], &genesisState)
	}

	return genesisState
}

func (gi *GenesisInfo) GetBaseBTCHeight() uint64 {
	return gi.baseBTCHeight
}

func (gi *GenesisInfo) GetEpochInterval() uint64 {
	return gi.epochInterval
}

func (gi *GenesisInfo) GetBLSKeySet() checkpointingtypes.ValidatorWithBlsKeySet {
	return gi.valSet
}

func (gi *GenesisInfo) GetCheckpointTag() string {
	return gi.checkpointTag
}
