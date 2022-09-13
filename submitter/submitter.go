package submitter

import (
	"errors"
	ckpttypes "github.com/babylonchain/babylon/x/checkpointing/types"
	"github.com/babylonchain/vigilante/btcclient"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"sync"

	"github.com/babylonchain/vigilante/babylonclient"
	"github.com/babylonchain/vigilante/config"
)

type Submitter struct {
	Cfg *config.SubmitterConfig

	btcClient         *btcclient.Client
	btcClientLock     sync.Mutex
	btcWallet         *btcclient.Client
	btcWalletLock     sync.Mutex
	babylonClient     *babylonclient.Client
	babylonClientLock sync.Mutex
	// TODO: add wallet client

	// Internal states of the reporter
	submitterAddress sdk.AccAddress
	account          string // wallet account

	wg      sync.WaitGroup
	started bool
	quit    chan struct{}
	quitMu  sync.Mutex

	// channel for relaying raw checkpoints to BTC
	rawCkptChan chan *ckpttypes.RawCheckpointWithMeta
}

func New(cfg *config.SubmitterConfig, btcClient *btcclient.Client, btcWallet *btcclient.Client, babylonClient *babylonclient.Client, addr sdk.AccAddress, account string) (*Submitter, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	// TODO: make use of BBN params

	return &Submitter{
		Cfg:              cfg,
		btcWallet:        btcClient,
		btcClient:        btcClient,
		babylonClient:    babylonClient,
		rawCkptChan:      make(chan *ckpttypes.RawCheckpointWithMeta, cfg.BufferSize),
		submitterAddress: addr,
		account:          account,
		quit:             make(chan struct{}),
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
	go s.rawCheckpointPoller()
	s.wg.Add(1)
	go s.sealedCkptHandler()

	log.Infof("Successfully created the vigilant submitter")
}

func (s *Submitter) GetBabylonClient() (*babylonclient.Client, error) {
	s.babylonClientLock.Lock()
	client := s.babylonClient
	s.babylonClientLock.Unlock()
	if client == nil {
		return nil, errors.New("Babylon client is inactive")
	}
	return client, nil
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
		s.babylonClientLock.Lock()
		if s.babylonClient != nil {
			s.babylonClient.Stop()
			s.babylonClient = nil
		}
		s.babylonClientLock.Unlock()
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
