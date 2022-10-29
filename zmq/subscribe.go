package zmq

import (
	"encoding/binary"
	"github.com/babylonchain/vigilante/types"
	"sync"
	"time"

	zmq "github.com/pebbe/zmq4"
)

// SequenceMsg is a subscription event coming from a "sequence" ZMQ message.
type SequenceMsg struct {
	Hash       [32]byte // use encoding/hex.EncodeToString() to get it into the RPC method string format.
	Event      types.EventType
	MempoolSeq uint64
}

type subscriptions struct {
	sync.RWMutex

	exited      chan struct{}
	zfront      *zmq.Socket
	latestEvent time.Time

	sequence []chan SequenceMsg
}

// SubscribeSequence subscribes to the ZMQ "sequence" messages as SequenceMsg items pushed onto the channel.
//
// Call cancel to cancel the subscription and let the client release the resources. The channel is closed
// when the subscription is canceled or when the client is closed.
func (bc *Client) SubscribeSequence() (subCh chan SequenceMsg, cancel func(), err error) {
	if bc.zsub == nil {
		err = ErrSubscribeDisabled
		return
	}
	bc.subs.Lock()
	select {
	case <-bc.subs.exited:
		err = ErrSubscribeExited
		bc.subs.Unlock()
		return
	default:
	}
	if len(bc.subs.sequence) == 0 {
		_, err = bc.subs.zfront.SendMessage("subscribe", "sequence")
		if err != nil {
			bc.subs.Unlock()
			return
		}
	}
	subCh = make(chan SequenceMsg, bc.Cfg.SubChannelBufferSize)
	bc.subs.sequence = append(bc.subs.sequence, subCh)
	bc.subs.Unlock()
	cancel = func() {
		err = bc.unsubscribeSequence(subCh)
		if err != nil {
			log.Errorf("Error unsubscribing from sequence: %v", err)
			return
		}
	}
	return
}

func (bc *Client) unsubscribeSequence(subCh chan SequenceMsg) (err error) {
	bc.subs.Lock()
	select {
	case <-bc.subs.exited:
		err = ErrSubscribeExited
		bc.subs.Unlock()
		return
	default:
	}
	for i, ch := range bc.subs.sequence {
		if ch == subCh {
			bc.subs.sequence = append(bc.subs.sequence[:i], bc.subs.sequence[i+1:]...)
			if len(bc.subs.sequence) == 0 {
				_, err = bc.subs.zfront.SendMessage("unsubscribe", "sequence")
			}
			break
		}
	}
	bc.subs.Unlock()
	close(subCh)
	return
}

func (bc *Client) zmqHandler() {
	defer bc.wg.Done()
	defer func(zsub *zmq.Socket) {
		err := zsub.Close()
		if err != nil {
			log.Errorf("Error closing ZMQ socket: %v", err)
		}
	}(bc.zsub)
	defer func(zback *zmq.Socket) {
		err := zback.Close()
		if err != nil {
			log.Errorf("Error closing ZMQ socket: %v", err)
		}
	}(bc.zback)

	poller := zmq.NewPoller()
	poller.Add(bc.zsub, zmq.POLLIN)
	poller.Add(bc.zback, zmq.POLLIN)
OUTER:
	for {
		// Wait forever until a message can be received or the context was cancelled.
		polled, err := poller.Poll(-1)
		if err != nil {
			break OUTER
		}

		for _, p := range polled {
			switch p.Socket {
			case bc.zsub:
				msg, err := bc.zsub.RecvMessage(0)
				if err != nil {
					break OUTER
				}
				bc.subs.latestEvent = time.Now()
				switch msg[0] {
				case "sequence":
					var sequenceMsg SequenceMsg
					copy(sequenceMsg.Hash[:], msg[1])
					switch msg[1][32] {
					case 'C':
						sequenceMsg.Event = types.BlockConnected
					case 'D':
						sequenceMsg.Event = types.BlockDisconnected
					case 'R':
						sequenceMsg.Event = types.TransactionRemoved
						sequenceMsg.MempoolSeq = binary.LittleEndian.Uint64([]byte(msg[1][33:]))
					case 'A':
						sequenceMsg.Event = types.TransactionAdded
						sequenceMsg.MempoolSeq = binary.LittleEndian.Uint64([]byte(msg[1][33:]))
					default:
						// This is a fault. Drop the message.
						continue
					}
					bc.subs.RLock()
					for _, ch := range bc.subs.sequence {
						select {
						case ch <- sequenceMsg:
						default:
							select {
							// Pop the oldest item and push the newest item (the user will miss a message).
							case _ = <-ch:
								ch <- sequenceMsg
							case ch <- sequenceMsg:
							default:
							}
						}
					}
					bc.subs.RUnlock()
				}

			case bc.zback:
				msg, err := bc.zback.RecvMessage(0)
				if err != nil {
					break OUTER
				}
				switch msg[0] {
				case "subscribe":
					if err := bc.zsub.SetSubscribe(msg[1]); err != nil {
						break OUTER
					}
				case "unsubscribe":
					if err := bc.zsub.SetUnsubscribe(msg[1]); err != nil {
						break OUTER
					}
				case "term":
					break OUTER
				}
			}
		}
	}

	bc.subs.Lock()
	close(bc.subs.exited)
	err := bc.subs.zfront.Close()
	if err != nil {
		log.Errorf("Error closing zfront: %v", err)
		return
	}
	// Close all subscriber channels, that will make them notice that we failed.
	if len(bc.subs.sequence) > 0 {
		err := bc.zsub.SetUnsubscribe("sequence")
		if err != nil {
			log.Errorf("Error unsubscribing from sequence: %v", err)
			return
		}
	}
	for _, ch := range bc.subs.sequence {
		close(ch)
	}
	bc.subs.Unlock()
}
