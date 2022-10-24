package types

import "github.com/btcsuite/btcd/wire"

type EventType int

const (
	// BlockDisconnected indicates the associated block was disconnected
	// from the main chain.
	BlockDisconnected EventType = iota

	// BlockConnected indicates the associated block was connected to the
	// main chain.
	BlockConnected
)

type BlockEvent struct {
	EventType EventType
	Height    int32
	Header    *wire.BlockHeader
}

func NewBlockEvent(eventType EventType, height int32, header *wire.BlockHeader) *BlockEvent {
	return &BlockEvent{
		EventType: eventType,
		Height:    height,
		Header:    header,
	}
}
