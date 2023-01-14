package monitor

import (
	"fmt"
	"github.com/btcsuite/btcd/wire"
	"sort"

	checkpointingtypes "github.com/babylonchain/babylon/x/checkpointing/types"
	bbnclient "github.com/babylonchain/rpc-client/client"
	"github.com/babylonchain/vigilante/config"
	"github.com/babylonchain/vigilante/monitor/btcscanner"
	"github.com/babylonchain/vigilante/monitor/querier"
	"github.com/babylonchain/vigilante/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

type Monitor struct {
	Cfg *config.MonitorConfig

	// BTCScanner scans BTC blocks for checkpoints
	BTCScanner btcscanner.Scanner
	// Querier queries epoch info from Babylon
	Querier *querier.Querier

	// curEpoch contains information of the current epoch for verification
	curEpoch *types.EpochInfo

	// tracks checkpoint records that have not been reported back to Babylon
	checkpointChecklist *types.CheckpointsBookkeeper
}

func New(cfg *config.MonitorConfig, genesisInfo *types.GenesisInfo, scanner btcscanner.Scanner, babylonClient bbnclient.BabylonClient) (*Monitor, error) {
	q := &querier.Querier{BabylonClient: babylonClient}

	checkpointZero, err := babylonClient.QueryRawCheckpoint(uint64(0))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch checkpoint for epoch 0: %w", err)
	}
	// genesis validator set needs to be sorted by address to respect the signing order
	sortedGenesisValSet := GetSortedValSet(genesisInfo.GetBLSKeySet())
	genesisEpoch := types.NewEpochInfo(
		uint64(0),
		sortedGenesisValSet,
		checkpointZero.Ckpt,
	)
	err = genesisEpoch.VerifyCheckpoint(checkpointZero.Ckpt)
	if err != nil {
		return nil, fmt.Errorf("checkpoint zero is invalid: %w", err)
	}

	return &Monitor{
		Querier:             q,
		BTCScanner:          scanner,
		Cfg:                 cfg,
		curEpoch:            genesisEpoch,
		checkpointChecklist: types.NewCheckpointsBookkeeper(),
	}, nil
}

// Start starts the verification core
func (m *Monitor) Start() {
	go m.BTCScanner.Start()

	go m.LivenessChecker()

	log.Info("the Monitor is started")
	for {
		select {
		case header := <-m.BTCScanner.GetHeadersChan():
			err := m.handleNewConfirmedHeader(header)
			if err != nil {
				log.Errorf("failed to handle BTC header %x: %s", header.BlockHash(), err.Error())
				break
			}
		case ckpt := <-m.BTCScanner.GetCheckpointsChan():
			err := m.handleNewConfirmedCheckpoint(ckpt)
			if err != nil {
				log.Errorf("failed to handler BTC raw checkpoint at epoch %d: %s", ckpt.EpochNum, err.Error())
			}

		}
	}
}

func (m *Monitor) handleNewConfirmedHeader(header *wire.BlockHeader) error {
	return m.checkHeaderConsistency(header)
}

func (m *Monitor) handleNewConfirmedCheckpoint(ckpt *types.CheckpointBTC) error {
	err := m.verifyCheckpoint(ckpt.RawCheckpoint)
	if err != nil {
		if sdkerrors.IsOf(err, types.ErrInconsistentLastCommitHash) {
			// stop verification if a valid BTC checkpoint on an inconsistent LastCommitHash is found
			// this means the ledger is on a fork
			return fmt.Errorf("verification failed at epoch %v: %w", m.GetCurrentEpoch(), err)
		}
		// skip the error if it is not ErrInconsistentLastCommitHash and verify the next BTC checkpoint
		log.Infof("invalid BTC checkpoint found at epoch %v: %s", m.GetCurrentEpoch(), err.Error())
		return nil
	}
	m.addCheckpointToCheckList(ckpt)
	log.Infof("checkpoint at epoch %v has passed the verification", m.GetCurrentEpoch())
	nextEpochNum := m.GetCurrentEpoch() + 1
	err = m.updateEpochInfo(nextEpochNum)
	if err != nil {
		return fmt.Errorf("cannot get information at epoch %v: %w", nextEpochNum, err)
	}

	return nil
}

func (m *Monitor) GetCurrentEpoch() uint64 {
	return m.curEpoch.GetEpochNumber()
}

func (m *Monitor) verifyCheckpoint(ckpt *checkpointingtypes.RawCheckpoint) error {
	return m.curEpoch.VerifyCheckpoint(ckpt)
}

func (m *Monitor) addCheckpointToCheckList(ckpt *types.CheckpointBTC) {
	record := types.NewCheckpointRecord(ckpt.RawCheckpoint, ckpt.BtcHeight)
	m.checkpointChecklist.Add(record)
}

func (m *Monitor) updateEpochInfo(epoch uint64) error {
	ei, err := m.Querier.QueryInfoForNextEpoch(epoch)
	if err != nil {
		return err
	}
	m.curEpoch = ei

	return nil
}

func (m *Monitor) checkHeaderConsistency(header *wire.BlockHeader) error {
	btcHeaderHash := header.BlockHash()

	log.Debugf("header for consistency check, hash %x", btcHeaderHash)

	consistent, err := m.Querier.ContainsBTCHeader(&btcHeaderHash)
	if err != nil {
		return err
	}
	if !consistent {
		return fmt.Errorf("BTC header %x does not exists on Babylon BTC light client", btcHeaderHash)
	}

	return nil
}

func GetSortedValSet(valSet checkpointingtypes.ValidatorWithBlsKeySet) checkpointingtypes.ValidatorWithBlsKeySet {
	sort.Slice(valSet, func(i, j int) bool {
		addri, err := sdk.ValAddressFromBech32(valSet.ValSet[i].ValidatorAddress)
		if err != nil {
			panic(fmt.Errorf("failed to parse validator address %v: %w", valSet.ValSet[i].ValidatorAddress, err))
		}
		addrj, err := sdk.ValAddressFromBech32(valSet.ValSet[j].ValidatorAddress)
		if err != nil {
			panic(fmt.Errorf("failed to parse validator address %v: %w", valSet.ValSet[j].ValidatorAddress, err))
		}

		return sdk.BigEndianToUint64(addri) < sdk.BigEndianToUint64(addrj)
	})

	return valSet
}

// Stop signals all vigilante goroutines to shutdown.
func (m *Monitor) Stop() {
	panic("implement graceful stop")
}
