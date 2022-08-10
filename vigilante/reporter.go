package vigilante

import (
	"errors"
	"sync"

	"github.com/babylonchain/vigilante/btcclient"
	"github.com/babylonchain/vigilante/config"
)

type Reporter struct {
	btcClient     *btcclient.Client
	btcClientLock sync.Mutex
	// TODO: add Babylon client

	wg sync.WaitGroup

	started bool
	quit    chan struct{}
	quitMu  sync.Mutex
}

func NewReporter(cfg *config.ReporterConfig, btcClient *btcclient.Client) (*Reporter, error) {
	return &Reporter{
		btcClient: btcClient,
		quit:      make(chan struct{}),
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

	log.Infof("Successfully created the vigilant reporter")

	// r.wg.Add(2)
	// go r.txCreator()
	// go r.walletLocker()
}

// SynchronizeRPC associates the vigilante with the consensus RPC client,
// synchronizes the vigilante with the latest changes to the blockchain, and
// continuously updates the vigilante through RPC notifications.
//
// This method is unstable and will be removed when all syncing logic is moved
// outside of the vigilante package.
func (r *Reporter) SynchronizeRPC(btcClient *btcclient.Client) {
	r.quitMu.Lock()
	select {
	case <-r.quit:
		r.quitMu.Unlock()
		return
	default:
	}
	r.quitMu.Unlock()

	// TODO: Ignoring the new client when one is already set breaks callers
	// who are replacing the client, perhaps after a disconnect.
	r.btcClientLock.Lock()
	if r.btcClient != nil {
		r.btcClientLock.Unlock()
		return
	}
	r.btcClient = btcClient
	r.btcClientLock.Unlock()

	// TODO: add internal logic of reporter
	// TODO: It would be preferable to either run these goroutines
	// separately from the vigilante (use vigilante mutator functions to
	// make changes from the RPC client) and not have to stop and
	// restart them each time the client disconnects and reconnets.
	// r.wg.Add(4)
	// go r.handleChainNotifications()
	// go r.rescanBatchHandler()
	// go r.rescanProgressHandler()
	// go r.rescanRPCHandler()
}

// requirebtcClient marks that a vigilante method can only be completed when the
// consensus RPC server is set.  This function and all functions that call it
// are unstable and will need to be moved when the syncing code is moved out of
// the vigilante.
func (r *Reporter) requireGetBtcClient() (*btcclient.Client, error) {
	r.btcClientLock.Lock()
	btcClient := r.btcClient
	r.btcClientLock.Unlock()
	if btcClient == nil {
		return nil, errors.New("blockchain RPC is inactive")
	}
	return btcClient, nil
}

// btcClient returns the optional consensus RPC client associated with the
// vigilante.
//
// This function is unstable and will be removed once sync logic is moved out of
// the vigilante.
func (r *Reporter) getBtcClient() *btcclient.Client {
	r.btcClientLock.Lock()
	btcClient := r.btcClient
	r.btcClientLock.Unlock()
	return btcClient
}

// quitChan atomically reads the quit channel.
func (r *Reporter) quitChan() <-chan struct{} {
	r.quitMu.Lock()
	c := r.quit
	r.quitMu.Unlock()
	return c
}

// Stop signals all vigilante goroutines to shutdown.
func (r *Reporter) Stop() {
	r.quitMu.Lock()
	quit := r.quit
	r.quitMu.Unlock()

	select {
	case <-quit:
	default:
		close(quit)
		r.btcClientLock.Lock()
		if r.btcClient != nil {
			r.btcClient.Stop()
			r.btcClient = nil
		}
		r.btcClientLock.Unlock()
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
	r.wg.Wait()
}
