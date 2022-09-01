package reporter

import (
	"errors"
	"github.com/babylonchain/vigilante/types"
	"sync"

	"github.com/babylonchain/vigilante/babylonclient"
	"github.com/babylonchain/vigilante/btcclient"
	"github.com/babylonchain/vigilante/config"
	"github.com/babylonchain/vigilante/netparams"
)

type Reporter struct {
	Cfg *config.ReporterConfig

	btcClient         *btcclient.Client
	btcClientLock     sync.Mutex
	babylonClient     *babylonclient.Client
	babylonClientLock sync.Mutex

	// Internal states of the reporter
	ckptSegmentPool types.CkptSegmentPool

	cache   *types.BTCCache
	wg      sync.WaitGroup
	started bool
	quit    chan struct{}
	quitMu  sync.Mutex
}

func New(cfg *config.ReporterConfig, btcClient *btcclient.Client, babylonClient *babylonclient.Client) (*Reporter, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	// initialise ckpt segment pool
	// TODO: bootstrapping
	params := netparams.GetBabylonParams(cfg.NetParams)
	pool := types.NewCkptSegmentPool(params.Tag, params.Version)
	cache := types.NewBTCCache(cfg.BTCCacheMaxEntries)

	return &Reporter{
		Cfg:             cfg,
		btcClient:       btcClient,
		babylonClient:   babylonClient,
		ckptSegmentPool: pool,
		cache:           cache,
		quit:            make(chan struct{}),
	}, nil
}

// Start starts the goroutines necessary to manage a vigilante.
func (r *Reporter) Start() {
	// initialize BTC Cache
	r.InitBTCCache()

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
	go r.indexedBlockHandler()

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

func (r *Reporter) InitBTCCache() {
	err := r.cache.Init(r.btcClient.Client)
	if err != nil {
		panic(err)
	}
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
