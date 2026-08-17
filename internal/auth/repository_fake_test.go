package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/Luzin7/vozzera-backend/internal/shared/httpx"
)

type fakeRepo struct {
	createUser        func(context.Context, CreateUserParams) (CreateUserRow, error)
	getUserByUsername func(context.Context, string) (GetUserByUsernameRow, error)
	getUserByID       func(context.Context, uuid.UUID) (User, error)
	insertSession     func(context.Context, InsertSessionParams) (InsertSessionRow, error)
	deleteSessionByID func(context.Context, uuid.UUID) error
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		createUser: func(context.Context, CreateUserParams) (CreateUserRow, error) {
			return CreateUserRow{}, errors.New("createUser não configurado")
		},
		getUserByUsername: func(context.Context, string) (GetUserByUsernameRow, error) {
			return GetUserByUsernameRow{}, errors.New("getUserByUsername não configurado")
		},
		getUserByID: func(context.Context, uuid.UUID) (User, error) {
			return User{}, errors.New("getUserByID não configurado")
		},
		insertSession: func(context.Context, InsertSessionParams) (InsertSessionRow, error) {
			return InsertSessionRow{}, errors.New("insertSession não configurado")
		},
		deleteSessionByID: func(context.Context, uuid.UUID) error {
			return errors.New("deleteSessionByID não configurado")
		},
	}
}

func (f *fakeRepo) CreateUser(ctx context.Context, arg CreateUserParams) (CreateUserRow, error) {
	return f.createUser(ctx, arg)
}

func (f *fakeRepo) GetUserByUsername(ctx context.Context, username string) (GetUserByUsernameRow, error) {
	return f.getUserByUsername(ctx, username)
}

func (f *fakeRepo) GetUserByID(ctx context.Context, id uuid.UUID) (User, error) {
	return f.getUserByID(ctx, id)
}

func (f *fakeRepo) InsertSession(ctx context.Context, arg InsertSessionParams) (InsertSessionRow, error) {
	return f.insertSession(ctx, arg)
}

func (f *fakeRepo) DeleteSessionByID(ctx context.Context, id uuid.UUID) error {
	return f.deleteSessionByID(ctx, id)
}

func assertStatus(t *testing.T, err error, want int) {
	t.Helper()
	var e *httpx.Error
	if !errors.As(err, &e) {
		t.Fatalf("erro não mapeado: %v", err)
	}
	if e.Status != want {
		t.Errorf("status = %d, want %d (%s)", e.Status, want, e.Message)
	}
}

var _ Repository = (*fakeRepo)(nil)
