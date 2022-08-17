package reporter

import (
	"errors"
	"sync"

	"github.com/babylonchain/vigilante/babylonclient"
	"github.com/babylonchain/vigilante/btcclient"
	"github.com/babylonchain/vigilante/config"
)

type Reporter struct {
	btcClient         *btcclient.Client
	btcClientLock     sync.Mutex
	babylonClient     *babylonclient.Client
	babylonClientLock sync.Mutex

	wg sync.WaitGroup

	started bool
	quit    chan struct{}
	quitMu  sync.Mutex
}

func New(cfg *config.ReporterConfig, btcClient *btcclient.Client, babylonClient *babylonclient.Client) (*Reporter, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &Reporter{
		btcClient:     btcClient,
		babylonClient: babylonClient,
		quit:          make(chan struct{}),
	}, nil
}

// Start starts the goroutines necessary to manage a vigilante.
func (r *Reporter) Start() {
	r.quitMu.Lock()
	select {
	case <-r.quit:
		// Restart the vigilante goroutines after shutdown finishes.
		r.WaitForShutdown()
		r.quit = make(chan struct{})
	default:
		// Ignore when the vigilante is still running.
		if r.started {
			r.quitMu.Unlock()
			return
		}
		r.started = true
	}
	r.quitMu.Unlock()

	go func() {
		for {
			select {
			case ib := <-r.btcClient.IndexedBlockChan:
				log.Infof("Start handling block %v from BTC client", ib.BlockHash())
				// dispatch the indexed block to the handler
				r.handleIndexedBlock(ib)
			case <-r.quitChan():
				// We have been asked to stop
				return
			}
		}
	}()

	log.Infof("Successfully started the vigilant reporter")
}

func (r *Reporter) GetBtcClient() (*btcclient.Client, error) {
	r.btcClientLock.Lock()
	btcClient := r.btcClient
	r.btcClientLock.Unlock()
	if btcClient == nil {
		return nil, errors.New("blockchain RPC is inactive")
	}
	return btcClient, nil
}

func (r *Reporter) MustGetBtcClient() *btcclient.Client {
	client, err := r.GetBtcClient()
	if err != nil {
		panic(err)
	}
	return client
}

func (r *Reporter) GetBabylonClient() (*babylonclient.Client, error) {
	r.babylonClientLock.Lock()
	client := r.babylonClient
	r.babylonClientLock.Unlock()
	if client == nil {
		return nil, errors.New("Babylon client is inactive")
	}
	return client, nil
}

func (r *Reporter) MustGetBabylonClient() *babylonclient.Client {
	client, err := r.GetBabylonClient()
	if err != nil {
		panic(err)
	}
	return client
}

// quitChan atomically reads the quit channel.
func (r *Reporter) quitChan() <-chan struct{} {
	r.quitMu.Lock()
	c := r.quit
	r.quitMu.Unlock()
	return c
}

// Stop signals all vigilante goroutines to shutdown.
// TODO: I don't see any code that produces stuff to the quit channel. How do all Goroutines get quit notifictaions and stop?
func (r *Reporter) Stop() {
	r.quitMu.Lock()
	quit := r.quit
	r.quitMu.Unlock()

	select {
	case <-quit:
	default:
		close(quit)
		// shutdown BTC client
		r.btcClientLock.Lock()
		if r.btcClient != nil {
			r.btcClient.Stop()
			r.btcClient = nil
		}
		r.btcClientLock.Unlock()
		// shutdown Babylon client
		r.babylonClientLock.Lock()
		if r.babylonClient != nil {
			r.babylonClient.Stop()
			r.babylonClient = nil
		}
		r.babylonClientLock.Unlock()
	}
}

// ShuttingDown returns whether the vigilante is currently in the process of
// shutting down or not.
func (r *Reporter) ShuttingDown() bool {
	select {
	case <-r.quitChan():
		return true
	default:
		return false
	}
}

// WaitForShutdown blocks until all vigilante goroutines have finished executing.
func (r *Reporter) WaitForShutdown() {
	r.btcClientLock.Lock()
	if r.btcClient != nil {
		r.btcClient.WaitForShutdown()
	}
	r.btcClientLock.Unlock()
	// TODO: let Babylon client WaitForShutDown
	r.wg.Wait()
}
