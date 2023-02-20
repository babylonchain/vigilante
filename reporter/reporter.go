package reporter

import (
	"errors"
	"sync"
	"time"

	bbnclient "github.com/babylonchain/rpc-client/client"
	"github.com/babylonchain/vigilante/btcclient"
	"github.com/babylonchain/vigilante/config"
	"github.com/babylonchain/vigilante/netparams"
	"github.com/babylonchain/vigilante/types"
)

type Reporter struct {
	Cfg *config.ReporterConfig

	btcClient         btcclient.BTCClient
	btcClientLock     sync.Mutex
	babylonClient     bbnclient.BabylonClient
	babylonClientLock sync.Mutex

	// retry attributes
	retrySleepTime    time.Duration
	maxRetrySleepTime time.Duration

	// Internal states of the reporter
	CheckpointCache               *types.CheckpointCache
	btcCache                      *types.BTCCache
	btcConfirmationDepth          uint64
	checkpointFinalizationTimeout uint64

	wg      sync.WaitGroup
	started bool
	quit    chan struct{}
	quitMu  sync.Mutex
}

func New(cfg *config.ReporterConfig, btcClient btcclient.BTCClient, babylonClient bbnclient.BabylonClient,
	retrySleepTime, maxRetrySleepTime time.Duration) (*Reporter, error) {
	// retrieve k and w within btccParams
	btccParams := babylonClient.MustQueryBTCCheckpointParams()
	k := btccParams.BtcConfirmationDepth
	w := btccParams.CheckpointFinalizationTimeout
	log.Infof("BTCCheckpoint parameters: (k, w) = (%d, %d)", k, w)
	// Note that BTC cache is initialised only after bootstrapping

	params := netparams.GetBabylonParams(cfg.NetParams, babylonClient.GetTagIdx())
	ckptCache := types.NewCheckpointCache(params.Tag, params.Version)

	return &Reporter{
		Cfg:                           cfg,
		retrySleepTime:                retrySleepTime,
		maxRetrySleepTime:             maxRetrySleepTime,
		btcClient:                     btcClient,
		babylonClient:                 babylonClient,
		CheckpointCache:               ckptCache,
		btcConfirmationDepth:          k,
		checkpointFinalizationTimeout: w,
		quit:                          make(chan struct{}),
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

	r.wg.Add(1)
	go r.blockEventHandler()

	log.Infof("Successfully started the vigilant reporter")
}

func (r *Reporter) GetBtcClient() (btcclient.BTCClient, error) {
	r.btcClientLock.Lock()
	btcClient := r.btcClient
	r.btcClientLock.Unlock()
	if btcClient == nil {
		return nil, errors.New("blockchain RPC is inactive")
	}
	return btcClient, nil
}

func (r *Reporter) MustGetBtcClient() btcclient.BTCClient {
	client, err := r.GetBtcClient()
	if err != nil {
		panic(err)
	}
	return client
}

func (r *Reporter) GetBabylonClient() (bbnclient.BabylonClient, error) {
	r.babylonClientLock.Lock()
	client := r.babylonClient
	r.babylonClientLock.Unlock()
	if client == nil {
		return nil, errors.New("Babylon client is inactive")
	}
	return client, nil
}

func (r *Reporter) MustGetBabylonClient() bbnclient.BabylonClient {
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
func (r *Reporter) Stop() {
	r.quitMu.Lock()
	quit := r.quit
	r.quitMu.Unlock()

	select {
	case <-quit:
	default:
		// closing the `quit` channel will trigger all select case `<-quit`,
		// and thus making all handler routines to break the for loop.
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

// ShuttingDown returns whether the vigilante is currently in the process of shutting down or not.
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
