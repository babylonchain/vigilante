package monitor

import (
	"encoding/hex"
	"fmt"
	"sort"
	"sync"

	"github.com/btcsuite/btcd/wire"
	"github.com/pkg/errors"
	"go.uber.org/atomic"
	"go.uber.org/zap"

	sdkerrors "cosmossdk.io/errors"
	checkpointingtypes "github.com/babylonchain/babylon/x/checkpointing/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonchain/vigilante/btcclient"
	"github.com/babylonchain/vigilante/config"
	"github.com/babylonchain/vigilante/metrics"
	"github.com/babylonchain/vigilante/monitor/btcscanner"
	"github.com/babylonchain/vigilante/types"
)

type Monitor struct {
	Cfg    *config.MonitorConfig
	ComCfg *config.CommonConfig
	logger *zap.SugaredLogger

	// BTCScanner scans BTC blocks for checkpoints
	BTCScanner btcscanner.Scanner
	// BBNQuerier queries epoch info from Babylon
	BBNQuerier BabylonQueryClient

	// curEpoch contains information of the current epoch for verification
	curEpoch *types.EpochInfo

	// tracks checkpoint records that have not been reported back to Babylon
	checkpointChecklist *types.CheckpointsBookkeeper

	metrics *metrics.MonitorMetrics

	wg      sync.WaitGroup
	started *atomic.Bool
	quit    chan struct{}
}

func New(
	cfg *config.MonitorConfig,
	comCfg *config.CommonConfig,
	parentLogger *zap.Logger,
	genesisInfo *types.GenesisInfo,
	bbnQueryClient BabylonQueryClient,
	btcClient btcclient.BTCClient,
	monitorMetrics *metrics.MonitorMetrics,
) (*Monitor, error) {
	logger := parentLogger.With(zap.String("module", "monitor"))
	// create BTC scanner
	checkpointTagBytes, err := hex.DecodeString(genesisInfo.GetCheckpointTag())
	if err != nil {
		panic(fmt.Errorf("invalid hex checkpoint tag: %w", err))
	}
	btcScanner, err := btcscanner.New(
		cfg,
		logger,
		btcClient,
		genesisInfo.GetBaseBTCHeight(),
		checkpointTagBytes,
	)
	if err != nil {
		panic(fmt.Errorf("failed to create BTC scanner: %w", err))
	}

	// genesis validator set needs to be sorted by address to respect the signing order
	sortedGenesisValSet := GetSortedValSet(genesisInfo.GetBLSKeySet())
	genesisEpoch := types.NewEpochInfo(
		uint64(1),
		sortedGenesisValSet,
	)

	return &Monitor{
		BBNQuerier:          bbnQueryClient,
		BTCScanner:          btcScanner,
		Cfg:                 cfg,
		ComCfg:              comCfg,
		logger:              logger.Sugar(),
		curEpoch:            genesisEpoch,
		checkpointChecklist: types.NewCheckpointsBookkeeper(),
		metrics:             monitorMetrics,
		quit:                make(chan struct{}),
		started:             atomic.NewBool(false),
	}, nil
}

func (m *Monitor) SetLogger(logger *zap.SugaredLogger) {
	m.logger = logger
}

// Start starts the verification core
func (m *Monitor) Start() {
	if m.started.Load() {
		m.logger.Info("the Monitor is already started")
		return
	}

	m.started.Store(true)
	m.logger.Info("the Monitor is started")

	// start Babylon RPC client
	if err := m.BBNQuerier.Start(); err != nil {
		m.logger.Fatalf("failed to start Babylon querier: %v", err)
	}

	// starting BTC scanner
	m.wg.Add(1)
	go m.runBTCScanner()

	if m.Cfg.EnableLivenessChecker {
		// starting liveness checker
		m.wg.Add(1)
		go m.runLivenessChecker()
	}

	for m.started.Load() {
		select {
		case <-m.quit:
			m.logger.Info("the monitor is stopping")
			m.started.Store(false)
		case header := <-m.BTCScanner.GetHeadersChan():
			err := m.handleNewConfirmedHeader(header)
			if err != nil {
				m.logger.Errorf("found invalid BTC header: %s", err.Error())
				m.metrics.InvalidBTCHeadersCounter.Inc()
			}
			m.metrics.ValidBTCHeadersCounter.Inc()
		case ckpt := <-m.BTCScanner.GetCheckpointsChan():
			err := m.handleNewConfirmedCheckpoint(ckpt)
			if err != nil {
				m.logger.Errorf("failed to handle BTC raw checkpoint at epoch %d: %s", ckpt.EpochNum(), err.Error())
				m.metrics.InvalidEpochsCounter.Inc()
			}
			m.metrics.ValidEpochsCounter.Inc()
		}
	}

	m.wg.Wait()
	m.logger.Info("the Monitor is stopped")
}

func (m *Monitor) runBTCScanner() {
	m.BTCScanner.Start()
	m.wg.Done()
}

func (m *Monitor) handleNewConfirmedHeader(header *wire.BlockHeader) error {
	return m.checkHeaderConsistency(header)
}

func (m *Monitor) handleNewConfirmedCheckpoint(ckpt *types.CheckpointRecord) error {
	err := m.VerifyCheckpoint(ckpt.RawCheckpoint)
	if err != nil {
		if sdkerrors.IsOf(err, types.ErrInconsistentBlockHash) {
			// also record conflicting checkpoints since we need to ensure that
			// alarm will be sent if conflicting checkpoints are censored
			if m.Cfg.EnableLivenessChecker {
				m.addCheckpointToCheckList(ckpt)
			}
			// stop verification if a valid BTC checkpoint on an inconsistent BlockHash is found
			// this means the ledger is on a fork
			return fmt.Errorf("verification failed at epoch %v: %w", m.GetCurrentEpoch(), err)
		}
		// skip the error if it is not ErrInconsistentBlockHash and verify the next BTC checkpoint
		m.logger.Infof("invalid BTC checkpoint found at epoch %v: %s", m.GetCurrentEpoch(), err.Error())
		return nil
	}

	if m.Cfg.EnableLivenessChecker {
		m.addCheckpointToCheckList(ckpt)
	}

	m.logger.Infof("checkpoint at epoch %v has passed the verification", m.GetCurrentEpoch())

	nextEpochNum := m.GetCurrentEpoch() + 1
	err = m.UpdateEpochInfo(nextEpochNum)
	if err != nil {
		return fmt.Errorf("failed to update information of epoch %d: %w", nextEpochNum, err)
	}

	return nil
}

func (m *Monitor) GetCurrentEpoch() uint64 {
	return m.curEpoch.GetEpochNumber()
}

// VerifyCheckpoint verifies the BTC checkpoint against the Babylon counterpart
func (m *Monitor) VerifyCheckpoint(btcCkpt *checkpointingtypes.RawCheckpoint) error {
	// check whether the epoch number of the checkpoint equals to the current epoch number
	if m.GetCurrentEpoch() != btcCkpt.EpochNum {
		return errors.Wrapf(types.ErrInvalidEpochNum, fmt.Sprintf("found a checkpoint with epoch %v, but the monitor expects epoch %v",
			btcCkpt.EpochNum, m.GetCurrentEpoch()))
	}
	// verify BLS sig of the BTC checkpoint
	err := m.curEpoch.VerifyMultiSig(btcCkpt)
	if err != nil {
		return fmt.Errorf("invalid BLS sig of BTC checkpoint at epoch %d: %w", m.GetCurrentEpoch(), err)
	}
	// query checkpoint from Babylon
	res, err := m.queryRawCheckpointWithRetry(btcCkpt.EpochNum)
	if err != nil {
		return fmt.Errorf("failed to query raw checkpoint from Babylon, epoch %v: %w", btcCkpt.EpochNum, err)
	}
	ckpt := res.RawCheckpoint.Ckpt
	// verify BLS sig of the raw checkpoint from Babylon
	err = m.curEpoch.VerifyMultiSig(ckpt)
	if err != nil {
		return fmt.Errorf("invalid BLS sig of Babylon raw checkpoint at epoch %d: %w", m.GetCurrentEpoch(), err)
	}
	// check whether the checkpoint from Babylon has the same BlockHash of the BTC checkpoint
	if !ckpt.BlockHash.Equal(*btcCkpt.BlockHash) {
		return errors.Wrapf(types.ErrInconsistentBlockHash, fmt.Sprintf("Babylon checkpoint's BlockHash %s, BTC checkpoint's BlockHash %s",
			ckpt.BlockHash.String(), btcCkpt.BlockHash))
	}
	return nil
}

func (m *Monitor) addCheckpointToCheckList(ckpt *types.CheckpointRecord) {
	record := types.NewCheckpointRecord(ckpt.RawCheckpoint, ckpt.FirstSeenBtcHeight)
	m.checkpointChecklist.Add(record)
}

func (m *Monitor) UpdateEpochInfo(epoch uint64) error {
	ei, err := m.QueryInfoForNextEpoch(epoch)
	if err != nil {
		return fmt.Errorf("failed to query information of the epoch %d: %w", epoch, err)
	}
	m.curEpoch = ei

	return nil
}

func (m *Monitor) checkHeaderConsistency(header *wire.BlockHeader) error {
	btcHeaderHash := header.BlockHash()

	res, err := m.queryContainsBTCBlockWithRetry(&btcHeaderHash)
	if err != nil {
		return err
	}
	if !res.Contains {
		return fmt.Errorf("BTC header %x does not exist on Babylon BTC light client", btcHeaderHash)
	}

	return nil
}

func GetSortedValSet(valSet checkpointingtypes.ValidatorWithBlsKeySet) checkpointingtypes.ValidatorWithBlsKeySet {
	sort.Slice(valSet.ValSet, func(i, j int) bool {
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

// Stop signals all vigilante goroutines to shut down.
func (m *Monitor) Stop() {
	close(m.quit)
	m.BTCScanner.Stop()
	// in e2e the test manager will share access to BBN querier and shut down
	// it earlier than monitor, so we need to check if it's running here
	if m.BBNQuerier.IsRunning() {
		if err := m.BBNQuerier.Stop(); err != nil {
			m.logger.Fatalf("failed to stop Babylon querier: %v", err)
		}
	}
}
