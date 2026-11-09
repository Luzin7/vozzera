package chat

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

type fakeRepo struct {
	getMessagesByRoom func(context.Context, GetMessagesByRoomParams) ([]GetMessagesByRoomRow, error)
	getMessagesBefore func(context.Context, GetMessagesByRoomBeforeParams) ([]GetMessagesByRoomBeforeRow, error)
	updateMessage     func(context.Context, UpdateMessageParams) (UpdateMessageRow, error)
	deleteMessage     func(context.Context, DeleteMessageParams) (DeleteMessageRow, error)
	createMessage     func(context.Context, CreateMessageParams) (CreateMessageRow, error)
	getReactionState  func(context.Context, GetReactionStateParams) (GetReactionStateRow, error)
	addReaction       func(context.Context, AddReactionParams) (int64, error)
	removeReaction    func(context.Context, RemoveReactionParams) (int64, error)
	getReactionsByIDs func(context.Context, []uuid.UUID) ([]GetReactionsByMessageIDsRow, error)
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		getMessagesByRoom: func(context.Context, GetMessagesByRoomParams) ([]GetMessagesByRoomRow, error) {
			return nil, errors.New("getMessagesByRoom não configurado")
		},
		getMessagesBefore: func(context.Context, GetMessagesByRoomBeforeParams) ([]GetMessagesByRoomBeforeRow, error) {
			return nil, errors.New("getMessagesBefore não configurado")
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
		getReactionState: func(context.Context, GetReactionStateParams) (GetReactionStateRow, error) {
			return GetReactionStateRow{}, errors.New("getReactionState não configurado")
		},
		addReaction: func(context.Context, AddReactionParams) (int64, error) {
			return 0, errors.New("addReaction não configurado")
		},
		removeReaction: func(context.Context, RemoveReactionParams) (int64, error) {
			return 0, errors.New("removeReaction não configurado")
		},
		getReactionsByIDs: func(context.Context, []uuid.UUID) ([]GetReactionsByMessageIDsRow, error) {
			return nil, nil
		},
	}
}

func (f *fakeRepo) GetMessagesByRoom(ctx context.Context, arg GetMessagesByRoomParams) ([]GetMessagesByRoomRow, error) {
	return f.getMessagesByRoom(ctx, arg)
}

func (f *fakeRepo) GetMessagesByRoomBefore(ctx context.Context, arg GetMessagesByRoomBeforeParams) ([]GetMessagesByRoomBeforeRow, error) {
	return f.getMessagesBefore(ctx, arg)
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

func (f *fakeRepo) GetReactionState(ctx context.Context, arg GetReactionStateParams) (GetReactionStateRow, error) {
	return f.getReactionState(ctx, arg)
}

func (f *fakeRepo) AddReaction(ctx context.Context, arg AddReactionParams) (int64, error) {
	return f.addReaction(ctx, arg)
}

func (f *fakeRepo) RemoveReaction(ctx context.Context, arg RemoveReactionParams) (int64, error) {
	return f.removeReaction(ctx, arg)
}

func (f *fakeRepo) GetReactionsByMessageIDs(ctx context.Context, messageIDs []uuid.UUID) ([]GetReactionsByMessageIDsRow, error) {
	return f.getReactionsByIDs(ctx, messageIDs)
}

var _ Repository = (*fakeRepo)(nil)
