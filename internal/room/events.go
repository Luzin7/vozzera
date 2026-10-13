package room

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
	"github.com/google/uuid"
)

const (
	EventRoomCreated = "room.created"
	EventRoomUpdated = "room.updated"
	EventRoomDeleted = "room.deleted"
)

type RoomPayload struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	HasVoice  bool      `json:"has_voice"`
	StaffOnly bool      `json:"staff_only"`
	CreatedAt time.Time `json:"created_at"`
}

type RoomDeletedPayload struct {
	ID    uuid.UUID `json:"id"`
	IsMod bool      `json:"is_mod"`
}

type roomAnnouncement struct {
	event string
	room  Room
}

type roomEventPublisher struct {
	publisher realtime.Publisher
}

func (p roomEventPublisher) created(ctx context.Context, room Room) error {
	announcement := roomAnnouncement{event: EventRoomCreated, room: room}
	if err := p.announce(ctx, realtime.StaffRoomListTopic, announcement); err != nil {
		return err
	}
	if room.StaffOnly {
		return nil
	}
	return p.announce(ctx, realtime.MemberRoomListTopic, announcement)
}

func (p roomEventPublisher) updated(ctx context.Context, room Room) error {
	announcement := roomAnnouncement{event: EventRoomUpdated, room: room}
	if err := p.announce(ctx, realtime.StaffRoomListTopic, announcement); err != nil {
		return err
	}
	if room.StaffOnly {
		return p.announceRemoval(ctx, realtime.MemberRoomListTopic, RoomDeletedPayload{ID: room.ID})
	}
	return p.announce(ctx, realtime.MemberRoomListTopic, announcement)
}

func (p roomEventPublisher) deleted(ctx context.Context, roomID uuid.UUID) error {
	payload := RoomDeletedPayload{ID: roomID, IsMod: true}
	topics := []realtime.Topic{
		realtime.RoomTopic(roomID),
		realtime.StaffRoomListTopic,
		realtime.MemberRoomListTopic,
	}
	for _, topic := range topics {
		if err := p.announceRemoval(ctx, topic, payload); err != nil {
			return err
		}
	}
	return nil
}

func (p roomEventPublisher) announce(ctx context.Context, topic realtime.Topic, announcement roomAnnouncement) error {
	room := announcement.room
	data, err := json.Marshal(RoomPayload{
		ID:        room.ID,
		Name:      room.Name,
		HasVoice:  room.HasVoice,
		StaffOnly: room.StaffOnly,
		CreatedAt: room.CreatedAt.Time,
	})
	if err != nil {
		return err
	}

	return p.publisher.Publish(ctx, topic, realtime.Envelope{
		V:     1,
		Type:  announcement.event,
		Topic: topic,
		TS:    time.Now(),
		Data:  data,
	})
}

func (p roomEventPublisher) announceRemoval(ctx context.Context, deliveryTopic realtime.Topic, payload RoomDeletedPayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	return p.publisher.Publish(ctx, deliveryTopic, realtime.Envelope{
		V:     1,
		Type:  EventRoomDeleted,
		Topic: realtime.RoomTopic(payload.ID),
		TS:    time.Now(),
		Data:  data,
	})
}
