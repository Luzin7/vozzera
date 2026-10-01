package room

import (
	"context"
	"errors"

	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
	"github.com/google/uuid"
)

type fakeRepo struct {
	listRooms   func(context.Context) ([]Room, error)
	getRoomByID func(context.Context, uuid.UUID) (Room, error)
	createRoom  func(context.Context, CreateRoomParams) (Room, error)
	updateRoom  func(context.Context, UpdateRoomParams) (Room, error)
	deleteRoom  func(context.Context, uuid.UUID) (Room, error)
	getUserRole func(context.Context, uuid.UUID) (string, error)
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		listRooms: func(context.Context) ([]Room, error) {
			return nil, errors.New("listRooms não configurado")
		},
		getRoomByID: func(context.Context, uuid.UUID) (Room, error) {
			return Room{}, errors.New("getRoomByID não configurado")
		},
		createRoom: func(context.Context, CreateRoomParams) (Room, error) {
			return Room{}, errors.New("createRoom não configurado")
		},
		updateRoom: func(context.Context, UpdateRoomParams) (Room, error) {
			return Room{}, errors.New("updateRoom não configurado")
		},
		deleteRoom: func(context.Context, uuid.UUID) (Room, error) {
			return Room{}, errors.New("deleteRoom não configurado")
		},
		getUserRole: func(context.Context, uuid.UUID) (string, error) {
			return "", errors.New("getUserRole não configurado")
		},
	}
}

func (f *fakeRepo) ListRooms(ctx context.Context) ([]Room, error) {
	return f.listRooms(ctx)
}

func (f *fakeRepo) GetRoomByID(ctx context.Context, id uuid.UUID) (Room, error) {
	return f.getRoomByID(ctx, id)
}

func (f *fakeRepo) CreateRoom(ctx context.Context, arg CreateRoomParams) (Room, error) {
	return f.createRoom(ctx, arg)
}

func (f *fakeRepo) UpdateRoom(ctx context.Context, arg UpdateRoomParams) (Room, error) {
	return f.updateRoom(ctx, arg)
}

func (f *fakeRepo) DeleteRoom(ctx context.Context, id uuid.UUID) (Room, error) {
	return f.deleteRoom(ctx, id)
}

func (f *fakeRepo) GetUserRole(ctx context.Context, id uuid.UUID) (string, error) {
	return f.getUserRole(ctx, id)
}

type fakePublisher struct {
	envelopes []realtime.Envelope
	topics    []realtime.Topic
}

func (f *fakePublisher) Publish(_ context.Context, topic realtime.Topic, env realtime.Envelope) error {
	f.topics = append(f.topics, topic)
	f.envelopes = append(f.envelopes, env)
	return nil
}

type fakeRevoker struct {
	topics []realtime.Topic
}

func (f *fakeRevoker) RevokeTopic(_ context.Context, topic realtime.Topic) error {
	f.topics = append(f.topics, topic)
	return nil
}

var (
	_ Repository         = (*fakeRepo)(nil)
	_ realtime.Publisher = (*fakePublisher)(nil)
	_ TopicRevoker       = (*fakeRevoker)(nil)
)
