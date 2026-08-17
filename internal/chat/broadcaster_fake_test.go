package chat

import (
	"github.com/google/uuid"
)

type fakeBroadcaster struct {
	events []OutboundEvent
	closed []uuid.UUID
}

func (f *fakeBroadcaster) Broadcast(event OutboundEvent) {
	f.events = append(f.events, event)
}

func (f *fakeBroadcaster) CloseRoom(roomID uuid.UUID) {
	f.closed = append(f.closed, roomID)
}

var _ RoomBroadcaster = (*fakeBroadcaster)(nil)
