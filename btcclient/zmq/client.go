package zmq

import (
	"errors"
	"sync"
	"sync/atomic"

	"github.com/pebbe/zmq4"
)

var (
	ErrSubscribeDisabled = errors.New("subscribe disabled (ZmqEndpoint was not set)")
	ErrSubscribeExited   = errors.New("subscription backend has exited")
)

// Client is a client that provides methods for interacting with zmq4.
// Must be created with New and destroyed with Close.
// Clients are safe for concurrent use by multiple goroutines.
type Client struct {
	closed int32 // Set atomically.
	wg     sync.WaitGroup
	quit   chan struct{}

	zmqEndpoint          string
	subChannelBufferSize int

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
func New(zmqEndpoint string, subChannelBufferSize int) (*Client, error) {
	var (
		zctx  *zmq4.Context
		zsub  *zmq4.Socket
		zback *zmq4.Socket
		err   error
		c     = &Client{
			quit:                 make(chan struct{}),
			subChannelBufferSize: subChannelBufferSize,
		}
	)

	// ZMQ Subscribe.
	zctx, err = zmq4.NewContext()
	if err != nil {
		return nil, err
	}

	zsub, err = zctx.NewSocket(zmq4.SUB)
	if err != nil {
		return nil, err
	}
	if err = zsub.Connect(zmqEndpoint); err != nil {
		return nil, err
	}

	zback, err = zctx.NewSocket(zmq4.PAIR)
	if err != nil {
		return nil, err
	}
	if err = zback.Bind("inproc://channel"); err != nil {
		return nil, err
	}

	zfront, err := zctx.NewSocket(zmq4.PAIR)
	if err != nil {
		return nil, err
	}
	if err = zfront.Connect("inproc://channel"); err != nil {
		return nil, err
	}

	c.zctx = zctx
	c.zsub = zsub
	c.subs.exited = make(chan struct{})
	c.subs.zfront = zfront
	c.zback = zback

	c.wg.Add(1)
	go c.zmqHandler()

	return c, nil
}

// Close terminates the client and releases resources.
func (c *Client) Close() (err error) {
	if !atomic.CompareAndSwapInt32(&c.closed, 0, 1) {
		return errors.New("client already closed")
	}
	if c.zctx != nil {
		c.zctx.SetRetryAfterEINTR(false)
		c.subs.Lock()
		select {
		case <-c.subs.exited:
		default:
			if _, err = c.subs.zfront.SendMessage("term"); err != nil {
				return err
			}
		}
		c.subs.Unlock()
		<-c.subs.exited
		err = c.zctx.Term()
	}
	close(c.quit)
	c.wg.Wait()
	return nil
}
