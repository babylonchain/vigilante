package submitter

import (
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/babylonchain/babylon/btctxformatter"
	"github.com/babylonchain/babylon/types/retry"
	btcctypes "github.com/babylonchain/babylon/x/btccheckpoint/types"
	"github.com/babylonchain/rpc-client/query"

	"github.com/babylonchain/vigilante/metrics"
	"github.com/babylonchain/vigilante/submitter/relayer"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonchain/vigilante/btcclient"
	"github.com/babylonchain/vigilante/config"
	"github.com/babylonchain/vigilante/submitter/poller"
)

type Submitter struct {
	Cfg *config.SubmitterConfig

	relayer *relayer.Relayer
	poller  *poller.Poller

	metrics *metrics.SubmitterMetrics

	wg      sync.WaitGroup
	started bool
	quit    chan struct{}
	quitMu  sync.Mutex
}

func New(
	cfg *config.SubmitterConfig,
	btcWallet btcclient.BTCWallet,
	queryClient query.BabylonQueryClient,
	submitterAddr sdk.AccAddress,
	retrySleepTime, maxRetrySleepTime time.Duration,
	submitterMetrics *metrics.SubmitterMetrics,
) (*Submitter, error) {
	var (
		btccheckpointParams *btcctypes.QueryParamsResponse
		err                 error
	)

	err = retry.Do(retrySleepTime, maxRetrySleepTime, func() error {
		btccheckpointParams, err = queryClient.BTCCheckpointParams()
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get checkpoint params: %w", err)
	}

	// get checkpoint tag
	checkpointTag, err := hex.DecodeString(btccheckpointParams.Params.CheckpointTag)
	if err != nil {
		return nil, fmt.Errorf("failed to decode checkpoint tag: %w", err)
	}

	p := poller.New(queryClient, cfg.BufferSize)

	est, err := relayer.NewFeeEstimator(btcWallet.GetBTCConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to create fee estimator: %w", err)
	}

	r := relayer.New(
		btcWallet,
		checkpointTag,
		btctxformatter.CurrentVersion,
		submitterAddr,
		metrics.NewRelayerMetrics(submitterMetrics.Registry),
		est,
		cfg,
	)

	return &Submitter{
		Cfg:     cfg,
		poller:  p,
		relayer: r,
		metrics: submitterMetrics,
		quit:    make(chan struct{}),
	}, nil
}

// Start starts the goroutines necessary to manage a vigilante.
func (s *Submitter) Start() {
	s.quitMu.Lock()
	select {
	case <-s.quit:
		// Restart the vigilante goroutines after shutdown finishes.
		s.WaitForShutdown()
		s.quit = make(chan struct{})
	default:
		// Ignore when the vigilante is still running.
		if s.started {
			s.quitMu.Unlock()
			return
		}
		s.started = true
	}
	s.quitMu.Unlock()

	s.wg.Add(1)
	// TODO: implement subscriber to the raw checkpoints
	// TODO: when bootstrapping,
	// - start subscribing raw checkpoints
	// - query/forward sealed raw checkpoints to BTC
	// - keep subscribing new raw checkpoints
	go s.pollCheckpoints()
	s.wg.Add(1)
	go s.processCheckpoints()

	// start to record time-related metrics
	s.metrics.RecordMetrics()

	log.Infof("Successfully created the vigilant submitter")
}

// quitChan atomically reads the quit channel.
func (s *Submitter) quitChan() <-chan struct{} {
	s.quitMu.Lock()
	c := s.quit
	s.quitMu.Unlock()
	return c
}

// Stop signals all vigilante goroutines to shutdown.
func (s *Submitter) Stop() {
	s.quitMu.Lock()
	quit := s.quit
	s.quitMu.Unlock()

	select {
	case <-quit:
	default:
		close(quit)
	}
}

// ShuttingDown returns whether the vigilante is currently in the process of
// shutting down or not.
func (s *Submitter) ShuttingDown() bool {
	select {
	case <-s.quitChan():
		return true
	default:
		return false
	}
}

// WaitForShutdown blocks until all vigilante goroutines have finished executing.
func (s *Submitter) WaitForShutdown() {
	s.wg.Wait()
}

func (s *Submitter) pollCheckpoints() {
	defer s.wg.Done()
	quit := s.quitChan()

	ticker := time.NewTicker(time.Duration(s.Cfg.PollingIntervalSeconds) * time.Second)

	for {
		select {
		case <-ticker.C:
			log.Info("Polling sealed raw checkpoints...")
			err := s.poller.PollSealedCheckpoints()
			if err != nil {
				log.Errorf("failed to query raw checkpoints: %v", err)
				continue
			}
			log.Debugf("Next polling happens in %v seconds", s.Cfg.PollingIntervalSeconds)
		case <-quit:
			// We have been asked to stop
			return
		}
	}
}

func (s *Submitter) processCheckpoints() {
	defer s.wg.Done()
	quit := s.quitChan()

	for {
		select {
		case ckpt := <-s.poller.GetSealedCheckpointChan():
			log.Infof("A sealed raw checkpoint for epoch %v is found", ckpt.Ckpt.EpochNum)
			err := s.relayer.SendCheckpointToBTC(ckpt)
			if err != nil {
				log.Errorf("Failed to submit the raw checkpoint for %v: %v", ckpt.Ckpt.EpochNum, err)
				s.metrics.FailedCheckpointsCounter.Inc()
			}
			s.metrics.SecondsSinceLastCheckpointGauge.Set(0)
		case <-quit:
			// We have been asked to stop
			return
		}
	}
}
