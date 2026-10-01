package chat

import (
	"context"
	"errors"
)

type fakeRepo struct {
	getMessagesByRoom func(context.Context, GetMessagesByRoomParams) ([]GetMessagesByRoomRow, error)
	updateMessage     func(context.Context, UpdateMessageParams) (UpdateMessageRow, error)
	deleteMessage     func(context.Context, DeleteMessageParams) (DeleteMessageRow, error)
	createMessage     func(context.Context, CreateMessageParams) (CreateMessageRow, error)
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
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
