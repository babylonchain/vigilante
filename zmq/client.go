package zmq

import (
	"errors"
	"sync"
	"sync/atomic"

	"github.com/pebbe/zmq4"
)

const DefaultSubChannelBufferSize = 2

var (
	ErrSubscribeDisabled = errors.New("subscribe disabled (ZmqPubAddress was not set)")
	ErrSubscribeExited   = errors.New("subscription backend has exited")
)

type Config struct {
	// ZmqPubAddress is the public address that the zmq4 instance uses for zmqpub,
	// corresponding to what is set when starting zmq4 through one or multiple of:
	// {-zmqpubhashtx=address -zmqpubhashblock=address -zmqpubrawblock=address -zmqpubrawtx=address -zmqpubsequence=address}.
	// Only a single address is supported in this client. Either use the same address for
	// all desired topics when starting zmq4, or create a seperate client for each address.
	//
	// Example: "tcp://8.8.8.8:1234"
	// More examples at: https://github.com/bitcoin/bitcoin/blob/master/doc/zmq.md (the host part in those examples
	// are local IPs and should be replaced with public IPs here on the client side)
	//
	// If ZmqPubAddress is not set then the Subscribe functions will return ErrSubscribeDisabled when called.
	ZmqPubAddress string

	// SubChannelBufferSize sets the number of entries that a subscription channel can hold
	// before dropping entries, if it is not drained fast enough.
	// If not set (or set to zero) then defaults to DefaultSubChannelBufferSize.
	SubChannelBufferSize int
}

// Client is a client that provides methods for interacting with zmq4.
// Must be created with New and destroyed with Close.
//
// Clients are safe for concurrent use by multiple goroutines.
type Client struct {
	closed int32 // Set atomically.
	wg     sync.WaitGroup
	quit   chan struct{}

	Cfg Config

	// ZMQ subscription related things.
	zctx *zmq4.Context
	zsub *zmq4.Socket
	subs subscriptions
	// subs.zfront --> zback is used like a channel to send messages to the zmqHandler goroutine.
	// Have to use zmq4 sockets in place of native channels for communication from
	// other functions to the goroutine, since it is constantly waiting on the zsub socket,
	// it can't select on a channel at the same time but can poll on multiple sockets.
	zback *zmq4.Socket
}

// New returns an initiated client, or an error.
// Missing RpcAddress in Config will disable the RPC methods, and missing ZmqPubAddress
// will disable the Subscribe methods.
// New does not try using the RPC connection and can't detect if the ZMQ connection works,
// you need to call Ready in order to check connection health.
func New() (*Client, error) {
	bc := &Client{
		quit: make(chan struct{}),
	}

	// ZMQ Subscribe.
	if bc.Cfg.ZmqPubAddress != "" {
		if bc.Cfg.SubChannelBufferSize == 0 {
			bc.Cfg.SubChannelBufferSize = DefaultSubChannelBufferSize
		}

		zctx, err := zmq4.NewContext()
		if err != nil {
			return nil, err
		}
		zsub, err := zctx.NewSocket(zmq4.SUB)
		if err != nil {
			return nil, err
		}
		if err := zsub.Connect(bc.Cfg.ZmqPubAddress); err != nil {
			return nil, err
		}
		zback, err := zctx.NewSocket(zmq4.PAIR)
		if err != nil {
			return nil, err
		}
		if err := zback.Bind("inproc://channel"); err != nil {
			return nil, err
		}
		zfront, err := zctx.NewSocket(zmq4.PAIR)
		if err != nil {
			return nil, err
		}
		if err := zfront.Connect("inproc://channel"); err != nil {
			return nil, err
		}

		bc.zctx = zctx
		bc.zsub = zsub
		bc.subs.exited = make(chan struct{})
		bc.subs.zfront = zfront
		bc.zback = zback

		bc.wg.Add(1)
		go bc.zmqHandler()
	}

	return bc, nil
}

// Close terminates the client and releases resources.
func (bc *Client) Close() (err error) {
	if !atomic.CompareAndSwapInt32(&bc.closed, 0, 1) {
		return errors.New("client already closed")
	}
	if bc.zctx != nil {
		bc.zctx.SetRetryAfterEINTR(false)
		bc.subs.Lock()
		select {
		case <-bc.subs.exited:
		default:
			if _, err = bc.subs.zfront.SendMessage("term"); err != nil {
				return err
			}
		}
		bc.subs.Unlock()
		<-bc.subs.exited
		err = bc.zctx.Term()
	}
	close(bc.quit)
	bc.wg.Wait()
	return nil
}
