package voice

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

type fakeRepo struct {
	getRoomByID    func(context.Context, uuid.UUID) (Room, error)
	listVoiceRooms func(context.Context) ([]Room, error)
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		getRoomByID: func(context.Context, uuid.UUID) (Room, error) {
			return Room{}, errors.New("getRoomByID não configurado")
		},
		listVoiceRooms: func(context.Context) ([]Room, error) {
			return nil, errors.New("listVoiceRooms não configurado")
		},
	}
}

func (f *fakeRepo) GetRoomByID(ctx context.Context, id uuid.UUID) (Room, error) {
	return f.getRoomByID(ctx, id)
}

func (f *fakeRepo) ListVoiceRooms(ctx context.Context) ([]Room, error) {
	return f.listVoiceRooms(ctx)
}

var _ Repository = (*fakeRepo)(nil)
