package chat

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

type fakeRepo struct {
	listRooms         func(context.Context) ([]Room, error)
	createRoom        func(context.Context, CreateRoomParams) (Room, error)
	updateRoom        func(context.Context, UpdateRoomParams) (Room, error)
	deleteRoom        func(context.Context, uuid.UUID) (Room, error)
	getMessagesByRoom func(context.Context, GetMessagesByRoomParams) ([]GetMessagesByRoomRow, error)
	updateMessage     func(context.Context, UpdateMessageParams) (UpdateMessageRow, error)
	deleteMessage     func(context.Context, DeleteMessageParams) (DeleteMessageRow, error)
	createMessage     func(context.Context, CreateMessageParams) (CreateMessageRow, error)
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		listRooms: func(context.Context) ([]Room, error) {
			return nil, errors.New("listRooms não configurado")
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
		getMessagesByRoom: func(context.Context, GetMessagesByRoomParams) ([]GetMessagesByRoomRow, error) {
			return nil, errors.New("getMessagesByRoom não configurado")
		},
		updateMessage: func(context.Context, UpdateMessageParams) (UpdateMessageRow, error) {
			return UpdateMessageRow{}, errors.New("updateMessage não configurado")
		},
		deleteMessage: func(context.Context, DeleteMessageParams) (DeleteMessageRow, error) {
			return DeleteMessageRow{}, errors.New("deleteMessage não configurado")
		},
		createMessage: func(context.Context, CreateMessageParams) (CreateMessageRow, error) {
			return CreateMessageRow{}, errors.New("createMessage não configurado")
		},
	}
}

func (f *fakeRepo) ListRooms(ctx context.Context) ([]Room, error) {
	return f.listRooms(ctx)
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

func (f *fakeRepo) GetMessagesByRoom(ctx context.Context, arg GetMessagesByRoomParams) ([]GetMessagesByRoomRow, error) {
	return f.getMessagesByRoom(ctx, arg)
}

func (f *fakeRepo) UpdateMessage(ctx context.Context, arg UpdateMessageParams) (UpdateMessageRow, error) {
	return f.updateMessage(ctx, arg)
}

func (f *fakeRepo) DeleteMessage(ctx context.Context, arg DeleteMessageParams) (DeleteMessageRow, error) {
	return f.deleteMessage(ctx, arg)
}

func (f *fakeRepo) CreateMessage(ctx context.Context, arg CreateMessageParams) (CreateMessageRow, error) {
	return f.createMessage(ctx, arg)
}

var _ Repository = (*fakeRepo)(nil)
