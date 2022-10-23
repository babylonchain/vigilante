package bitcoind

import (
	"encoding/binary"
	"sync"
	"time"

	zmq "github.com/pebbe/zmq4"
)

// HashMsg is a subscription event coming from a "hash"-type ZMQ message.
type HashMsg struct {
	Hash [32]byte // use encoding/hex.EncodeToString() to get it into the RPC method string format.
	Seq  uint32
}

// RawMsg is a subscription event coming from a "raw"-type ZMQ message.
type RawMsg struct {
	Serialized []byte // use encoding/hex.EncodeToString() to get it into the RPC method string format.
	Seq        uint32
}

// SequenceMsg is a subscription event coming from a "sequence" ZMQ message.
type SequenceMsg struct {
	Hash       [32]byte // use encoding/hex.EncodeToString() to get it into the RPC method string format.
	Event      SequenceEvent
	MempoolSeq uint64
}

// SequenceEvent is an enum describing what event triggered the sequence message.
type SequenceEvent int

const (
	Invalid            SequenceEvent = iota
	BlockConnected                   // Blockhash connected
	BlockDisconnected                // Blockhash disconnected
	TransactionRemoved               // Transactionhash removed from mempool for non-block inclusion reason
	TransactionAdded                 // Transactionhash added mempool
)

func (se SequenceEvent) String() string {
	return [...]string{"Invalid", "Blockhash connected", "Blockhash disconnected",
		"Transactionhash removed from mempool for non-block inclusion reason", "Transactionhash added mempool"}[se]
}

type subscriptions struct {
	sync.RWMutex

	exited      chan struct{}
	zfront      *zmq.Socket
	latestEvent time.Time

	hashTx    [](chan HashMsg)
	hashBlock [](chan HashMsg)
	rawTx     [](chan RawMsg)
	rawBlock  [](chan RawMsg)
	sequence  [](chan SequenceMsg)
}

// SubscribeHashTx subscribes to the ZMQ "hashtx" messages as HashMsg items pushed onto the channel.
//
// Call cancel to cancel the subscription and let the client release the resources. The channel is closed
// when the subscription is canceled or when the client is closed.
func (bc *BitcoindClient) SubscribeHashTx() (subCh chan HashMsg, cancel func(), err error) {
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
	if len(bc.subs.hashTx) == 0 {
		_, err = bc.subs.zfront.SendMessage("subscribe", "hashtx")
		if err != nil {
			bc.subs.Unlock()
			return
		}
	}
	subCh = make(chan HashMsg, bc.Cfg.SubChannelBufferSize)
	bc.subs.hashTx = append(bc.subs.hashTx, subCh)
	bc.subs.Unlock()
	cancel = func() { bc.unsubscribeHashTx(subCh) }
	return
}

func (bc *BitcoindClient) unsubscribeHashTx(subCh chan HashMsg) (err error) {
	bc.subs.Lock()
	select {
	case <-bc.subs.exited:
		err = ErrSubscribeExited
		bc.subs.Unlock()
		return
	default:
	}
	for i, ch := range bc.subs.hashTx {
		if ch == subCh {
			bc.subs.hashTx = append(bc.subs.hashTx[:i], bc.subs.hashTx[i+1:]...)
			if len(bc.subs.hashTx) == 0 {
				_, err = bc.subs.zfront.SendMessage("unsubscribe", "hashtx")
			}
			break
		}
	}
	bc.subs.Unlock()
	close(subCh)
	return
}

// SubscribeHashBlock subscribes to the ZMQ "hashblock" messages as HashMsg items pushed onto the channel.
//
// Call cancel to cancel the subscription and let the client release the resources. The channel is closed
// when the subscription is canceled or when the client is closed.
func (bc *BitcoindClient) SubscribeHashBlock() (subCh chan HashMsg, cancel func(), err error) {
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
	if len(bc.subs.hashBlock) == 0 {
		_, err = bc.subs.zfront.SendMessage("subscribe", "hashblock")
		if err != nil {
			bc.subs.Unlock()
			return
		}
	}
	subCh = make(chan HashMsg, bc.Cfg.SubChannelBufferSize)
	bc.subs.hashBlock = append(bc.subs.hashBlock, subCh)
	bc.subs.Unlock()
	cancel = func() { bc.unsubscribeHashBlock(subCh) }
	return
}

func (bc *BitcoindClient) unsubscribeHashBlock(subCh chan HashMsg) (err error) {
	bc.subs.Lock()
	select {
	case <-bc.subs.exited:
		err = ErrSubscribeExited
		bc.subs.Unlock()
		return
	default:
	}
	for i, ch := range bc.subs.hashBlock {
		if ch == subCh {
			bc.subs.hashBlock = append(bc.subs.hashBlock[:i], bc.subs.hashBlock[i+1:]...)
			if len(bc.subs.hashBlock) == 0 {
				_, err = bc.subs.zfront.SendMessage("unsubscribe", "hashblock")
			}
			break
		}
	}
	bc.subs.Unlock()
	close(subCh)
	return
}

// SubscribeRawTx subscribes to the ZMQ "rawtx" messages as RawMsg items pushed onto the channel.
//
// Call cancel to cancel the subscription and let the client release the resources. The channel is closed
// when the subscription is canceled or when the client is closed.
func (bc *BitcoindClient) SubscribeRawTx() (subCh chan RawMsg, cancel func(), err error) {
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
	if len(bc.subs.rawTx) == 0 {
		_, err = bc.subs.zfront.SendMessage("subscribe", "rawtx")
		if err != nil {
			bc.subs.Unlock()
			return
		}
	}
	subCh = make(chan RawMsg, bc.Cfg.SubChannelBufferSize)
	bc.subs.rawTx = append(bc.subs.rawTx, subCh)
	bc.subs.Unlock()
	cancel = func() { bc.unsubscribeRawTx(subCh) }
	return
}

func (bc *BitcoindClient) unsubscribeRawTx(subCh chan RawMsg) (err error) {
	bc.subs.Lock()
	select {
	case <-bc.subs.exited:
		err = ErrSubscribeExited
		bc.subs.Unlock()
		return
	default:
	}
	for i, ch := range bc.subs.rawTx {
		if ch == subCh {
			bc.subs.rawTx = append(bc.subs.rawTx[:i], bc.subs.rawTx[i+1:]...)
			if len(bc.subs.rawTx) == 0 {
				_, err = bc.subs.zfront.SendMessage("unsubscribe", "rawtx")
			}
			break
		}
	}
	bc.subs.Unlock()
	close(subCh)
	return
}

// SubscribeRawBlock subscribes to the ZMQ "rawblock" messages as RawMsg items pushed onto the channel.
//
// Call cancel to cancel the subscription and let the client release the resources. The channel is closed
// when the subscription is canceled or when the client is closed.
func (bc *BitcoindClient) SubscribeRawBlock() (subCh chan RawMsg, cancel func(), err error) {
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
	if len(bc.subs.rawBlock) == 0 {
		_, err = bc.subs.zfront.SendMessage("subscribe", "rawblock")
		if err != nil {
			bc.subs.Unlock()
			return
		}
	}
	subCh = make(chan RawMsg, bc.Cfg.SubChannelBufferSize)
	bc.subs.rawBlock = append(bc.subs.rawBlock, subCh)
	bc.subs.Unlock()
	cancel = func() { bc.unsubscribeRawBlock(subCh) }
	return
}

func (bc *BitcoindClient) unsubscribeRawBlock(subCh chan RawMsg) (err error) {
	bc.subs.Lock()
	select {
	case <-bc.subs.exited:
		err = ErrSubscribeExited
		bc.subs.Unlock()
		return
	default:
	}
	for i, ch := range bc.subs.rawBlock {
		if ch == subCh {
			bc.subs.rawBlock = append(bc.subs.rawBlock[:i], bc.subs.rawBlock[i+1:]...)
			if len(bc.subs.rawBlock) == 0 {
				_, err = bc.subs.zfront.SendMessage("unsubscribe", "rawblock")
			}
			break
		}
	}
	bc.subs.Unlock()
	close(subCh)
	return
}

// SubscribeSequence subscribes to the ZMQ "sequence" messages as SequenceMsg items pushed onto the channel.
//
// Call cancel to cancel the subscription and let the client release the resources. The channel is closed
// when the subscription is canceled or when the client is closed.
func (bc *BitcoindClient) SubscribeSequence() (subCh chan SequenceMsg, cancel func(), err error) {
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
	cancel = func() { bc.unsubscribeSequence(subCh) }
	return
}

func (bc *BitcoindClient) unsubscribeSequence(subCh chan SequenceMsg) (err error) {
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

func (bc *BitcoindClient) zmqHandler() {
	defer bc.wg.Done()
	defer bc.zsub.Close()
	defer bc.zback.Close()

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
				case "hashtx":
					var hashMsg HashMsg
					copy(hashMsg.Hash[:], msg[1])
					hashMsg.Seq = binary.LittleEndian.Uint32([]byte(msg[2]))
					bc.subs.RLock()
					for _, ch := range bc.subs.hashTx {
						select {
						case ch <- hashMsg:
						default:
							select {
							// Pop the oldest item and push the newest item (the user will miss a message).
							case _ = <-ch:
								ch <- hashMsg
							case ch <- hashMsg:
							default:
							}
						}
					}
					bc.subs.RUnlock()
				case "hashblock":
					var hashMsg HashMsg
					copy(hashMsg.Hash[:], msg[1])
					hashMsg.Seq = binary.LittleEndian.Uint32([]byte(msg[2]))
					bc.subs.RLock()
					for _, ch := range bc.subs.hashBlock {
						select {
						case ch <- hashMsg:
						default:
							select {
							// Pop the oldest item and push the newest item (the user will miss a message).
							case _ = <-ch:
								ch <- hashMsg
							case ch <- hashMsg:
							default:
							}
						}
					}
					bc.subs.RUnlock()
				case "rawtx":
					var rawMsg RawMsg
					rawMsg.Serialized = []byte(msg[1])
					rawMsg.Seq = binary.LittleEndian.Uint32([]byte(msg[2]))
					bc.subs.RLock()
					for _, ch := range bc.subs.rawTx {
						select {
						case ch <- rawMsg:
						default:
							select {
							// Pop the oldest item and push the newest item (the user will miss a message).
							case _ = <-ch:
								ch <- rawMsg
							case ch <- rawMsg:
							default:
							}
						}
					}
					bc.subs.RUnlock()
				case "rawblock":
					var rawMsg RawMsg
					rawMsg.Serialized = []byte(msg[1])
					rawMsg.Seq = binary.LittleEndian.Uint32([]byte(msg[2]))
					bc.subs.RLock()
					for _, ch := range bc.subs.rawBlock {
						select {
						case ch <- rawMsg:
						default:
							select {
							// Pop the oldest item and push the newest item (the user will miss a message).
							case _ = <-ch:
								ch <- rawMsg
							case ch <- rawMsg:
							default:
							}
						}
					}
					bc.subs.RUnlock()
				case "sequence":
					var sequenceMsg SequenceMsg
					copy(sequenceMsg.Hash[:], msg[1])
					switch msg[1][32] {
					case 'C':
						sequenceMsg.Event = BlockConnected
					case 'D':
						sequenceMsg.Event = BlockDisconnected
					case 'R':
						sequenceMsg.Event = TransactionRemoved
						sequenceMsg.MempoolSeq = binary.LittleEndian.Uint64([]byte(msg[1][33:]))
					case 'A':
						sequenceMsg.Event = TransactionAdded
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
	bc.subs.zfront.Close()
	// Close all subscriber channels, that will make them notice that we failed.
	if len(bc.subs.hashTx) > 0 {
		bc.zsub.SetUnsubscribe("hashtx")
	}
	for _, ch := range bc.subs.hashTx {
		close(ch)
	}
	if len(bc.subs.hashBlock) > 0 {
		bc.zsub.SetUnsubscribe("hashblock")
	}
	for _, ch := range bc.subs.hashBlock {
		close(ch)
	}
	if len(bc.subs.rawTx) > 0 {
		bc.zsub.SetUnsubscribe("rawtx")
	}
	for _, ch := range bc.subs.rawTx {
		close(ch)
	}
	if len(bc.subs.rawBlock) > 0 {
		bc.zsub.SetUnsubscribe("rawblock")
	}
	for _, ch := range bc.subs.rawBlock {
		close(ch)
	}
	if len(bc.subs.sequence) > 0 {
		bc.zsub.SetUnsubscribe("sequence")
	}
	for _, ch := range bc.subs.sequence {
		close(ch)
	}
	bc.subs.Unlock()
}
