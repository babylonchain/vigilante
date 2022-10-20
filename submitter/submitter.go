package submitter

import (
	"errors"
	"github.com/babylonchain/vigilante/submitter/relayer"
	"sync"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonchain/vigilante/babylonclient"
	"github.com/babylonchain/vigilante/btcclient"
	"github.com/babylonchain/vigilante/config"
	"github.com/babylonchain/vigilante/submitter/poller"
)

type Submitter struct {
	Cfg *config.SubmitterConfig

	relayer     *relayer.Relayer
	relayerLock sync.Mutex
	poller      *poller.Poller
	pollerLock  sync.Mutex

	wg      sync.WaitGroup
	started bool
	quit    chan struct{}
	quitMu  sync.Mutex
}

func New(cfg *config.SubmitterConfig, btcWallet *btcclient.Client, babylonClient *babylonclient.Client) (*Submitter, error) {
	bbnAddr, err := sdk.AccAddressFromBech32(babylonClient.Cfg.SubmitterAddress)
	if err != nil {
		return nil, err
	}

	p := poller.New(babylonClient, cfg.BufferSize)
	r := relayer.New(btcWallet,
		cfg.GetTag(p.GetTagIdx()),
		cfg.GetVersion(),
		bbnAddr,
		cfg.ResendIntervalSeconds,
	)

	return &Submitter{
		Cfg:     cfg,
		poller:  p,
		relayer: r,
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

	log.Infof("Successfully created the vigilant submitter")
}

func (s *Submitter) GetBabylonClient() (*babylonclient.Client, error) {
	s.pollerLock.Lock()
	client := s.poller.BabylonClient
	s.pollerLock.Unlock()
	if client == nil {
		return nil, errors.New("Babylon client is inactive")
	}
	return client.(*babylonclient.Client), nil
}

func (s *Submitter) MustGetBabylonClient() *babylonclient.Client {
	client, err := s.GetBabylonClient()
	if err != nil {
		panic(err)
	}
	return client
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
		// shutdown Babylon client
		s.pollerLock.Lock()
		s.poller.Stop()
		s.pollerLock.Unlock()
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
			log.Debugf("Next polling happens in %v seconds", s.Cfg.PollingIntervalSeconds)
			if err != nil {
				log.Errorf("failed to query raw checkpoints: %v", err)
				continue
			}
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
			err := s.relayer.TryAndSendCheckpointToBTC(ckpt)
			if err != nil {
				log.Errorf("Failed to submit the raw checkpoint for %v: %v", ckpt.Ckpt.EpochNum, err)
			}
		case <-quit:
			// We have been asked to stop
			return
		}
	}
}
