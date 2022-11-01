package zmq

import (
	"encoding/hex"
	"sync"
	"time"

	"github.com/babylonchain/vigilante/types"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	zmq "github.com/pebbe/zmq4"
)

// SequenceMsg is a subscription event coming from a "sequence" ZMQ message.
type SequenceMsg struct {
	Hash  [32]byte // use encoding/hex.EncodeToString() to get it into the RPC method string format.
	Event types.EventType
}

type subscriptions struct {
	sync.RWMutex

	exited      chan struct{}
	zfront      *zmq.Socket
	latestEvent time.Time
	active      bool

	sequence []chan *SequenceMsg
}

// SubscribeSequence subscribes to the ZMQ "sequence" messages as SequenceMsg items pushed onto the channel.
// Call cancel to cancel the subscription and let the client release the resources. The channel is closed
// when the subscription is canceled or when the client is closed.
func (c *Client) SubscribeSequence() (err error) {
	if c.zsub == nil {
		err = ErrSubscribeDisabled
		return
	}
	c.subs.Lock()
	select {
	case <-c.subs.exited:
		err = ErrSubscribeExited
		c.subs.Unlock()
		return
	default:
	}

	if c.subs.active {
		err = ErrSubscriptionAlreadyActive
		return
	}

	_, err = c.subs.zfront.SendMessage("subscribe", "sequence")
	if err != nil {
		c.subs.Unlock()
		return
	}
	c.subs.active = true

	c.subs.Unlock()
	return
}

func (c *Client) zmqHandler() {
	defer c.wg.Done()
	defer func(zsub *zmq.Socket) {
		err := zsub.Close()
		if err != nil {
			log.Errorf("Error closing ZMQ socket: %v", err)
		}
	}(c.zsub)
	defer func(zback *zmq.Socket) {
		err := zback.Close()
		if err != nil {
			log.Errorf("Error closing ZMQ socket: %v", err)
		}
	}(c.zback)

	poller := zmq.NewPoller()
	poller.Add(c.zsub, zmq.POLLIN)
	poller.Add(c.zback, zmq.POLLIN)
OUTER:
	for {
		// Wait forever until a message can be received or the context was cancelled.
		polled, err := poller.Poll(-1)
		if err != nil {
			break OUTER
		}

		for _, p := range polled {
			switch p.Socket {
			case c.zsub:
				msg, err := c.zsub.RecvMessage(0)
				if err != nil {
					break OUTER
				}
				c.subs.latestEvent = time.Now()
				switch msg[0] {
				case "sequence":
					var sequenceMsg SequenceMsg
					copy(sequenceMsg.Hash[:], msg[1])
					switch msg[1][32] {
					case 'C':
						sequenceMsg.Event = types.BlockConnected
					case 'D':
						sequenceMsg.Event = types.BlockDisconnected
					default:
						// not interested in other events
						continue
					}

					c.sendBlockEvent(sequenceMsg.Hash[:], sequenceMsg.Event)
				}

			case c.zback:
				msg, err := c.zback.RecvMessage(0)
				if err != nil {
					break OUTER
				}
				switch msg[0] {
				case "subscribe":
					if err := c.zsub.SetSubscribe(msg[1]); err != nil {
						break OUTER
					}
				case "term":
					break OUTER
				}
			}
		}
	}

	c.subs.Lock()
	close(c.subs.exited)
	err := c.subs.zfront.Close()
	if err != nil {
		log.Errorf("Error closing zfront: %v", err)
		return
	}
	// Close all subscriber channels.
	if c.subs.active {
		err = c.zsub.SetUnsubscribe("sequence")
		if err != nil {
			log.Errorf("Error unsubscribing from sequence: %v", err)
			return
		}
	}

	c.subs.Unlock()
}

func (c *Client) sendBlockEvent(hash []byte, event types.EventType) {
	blockHashStr := hex.EncodeToString(hash[:])
	blockHash, err := chainhash.NewHashFromStr(blockHashStr)
	if err != nil {
		log.Errorf("Failed to parse block hash %v: %v", blockHashStr, err)
		panic(err)
	}

	log.Infof("Received zmq sequence message for block %v", blockHashStr)

	ib, _, err := c.getBlockByHash(blockHash)
	if err != nil {
		log.Errorf("Failed to get block %v from BTC client: %v", blockHash, err)
		panic(err)
	}

	c.blockEventChan <- types.NewBlockEvent(event, ib.Height, ib.Header)
}

func (c *Client) getBlockByHash(blockHash *chainhash.Hash) (*types.IndexedBlock, *wire.MsgBlock, error) {
	blockInfo, err := c.rpcClient.GetBlockVerbose(blockHash)
	if err != nil {
		return nil, nil, err
	}

	mBlock, err := c.rpcClient.GetBlock(blockHash)
	if err != nil {
		return nil, nil, err
	}

	btcTxs := types.GetWrappedTxs(mBlock)
	return types.NewIndexedBlock(int32(blockInfo.Height), &mBlock.Header, btcTxs), mBlock, nil
}
