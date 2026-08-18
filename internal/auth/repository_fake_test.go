package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/Luzin7/vozzera-backend/internal/shared/httpx"
)

type fakeRepo struct {
	createUser                        func(context.Context, CreateUserParams) (CreateUserRow, error)
	getUserByUsername                 func(context.Context, string) (GetUserByUsernameRow, error)
	getUserByID                       func(context.Context, uuid.UUID) (User, error)
	getUserByEmail                    func(context.Context, string) (GetUserByEmailRow, error)
	getUserByIdentifier               func(context.Context, GetUserByIdentifierParams) (User, error)
	insertSession                     func(context.Context, InsertSessionParams) (InsertSessionRow, error)
	deleteSessionByID                 func(context.Context, uuid.UUID) error
	deleteSessionsByUser              func(context.Context, uuid.UUID) error
	updateUserPassword                func(context.Context, UpdateUserPasswordParams) error
	updateUserEmail                   func(context.Context, UpdateUserEmailParams) error
	insertPasswordResetToken          func(context.Context, InsertPasswordResetTokenParams) (PasswordResetToken, error)
	getPasswordResetTokenByHash       func(context.Context, string) (GetPasswordResetTokenByHashRow, error)
	deletePasswordResetToken          func(context.Context, uuid.UUID) error
	cleanupExpiredPasswordResetTokens func(context.Context) error
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
		getUserByEmail: func(context.Context, string) (GetUserByEmailRow, error) {
			return GetUserByEmailRow{}, errors.New("getUserByEmail não configurado")
		},
		getUserByIdentifier: func(context.Context, GetUserByIdentifierParams) (User, error) {
			return User{}, errors.New("getUserByIdentifier não configurado")
		},
		insertSession: func(context.Context, InsertSessionParams) (InsertSessionRow, error) {
			return InsertSessionRow{}, errors.New("insertSession não configurado")
		},
		deleteSessionByID: func(context.Context, uuid.UUID) error {
			return errors.New("deleteSessionByID não configurado")
		},
		deleteSessionsByUser: func(context.Context, uuid.UUID) error {
			return errors.New("deleteSessionsByUser não configurado")
		},
		updateUserPassword: func(context.Context, UpdateUserPasswordParams) error {
			return errors.New("updateUserPassword não configurado")
		},
		updateUserEmail: func(context.Context, UpdateUserEmailParams) error {
			return errors.New("updateUserEmail não configurado")
		},
		insertPasswordResetToken: func(context.Context, InsertPasswordResetTokenParams) (PasswordResetToken, error) {
			return PasswordResetToken{}, errors.New("insertPasswordResetToken não configurado")
		},
		getPasswordResetTokenByHash: func(context.Context, string) (GetPasswordResetTokenByHashRow, error) {
			return GetPasswordResetTokenByHashRow{}, errors.New("getPasswordResetTokenByHash não configurado")
		},
		deletePasswordResetToken: func(context.Context, uuid.UUID) error {
			return errors.New("deletePasswordResetToken não configurado")
		},
		cleanupExpiredPasswordResetTokens: func(context.Context) error {
			return errors.New("cleanupExpiredPasswordResetTokens não configurado")
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

func (f *fakeRepo) GetUserByEmail(ctx context.Context, email string) (GetUserByEmailRow, error) {
	return f.getUserByEmail(ctx, email)
}

func (f *fakeRepo) GetUserByIdentifier(ctx context.Context, arg GetUserByIdentifierParams) (User, error) {
	return f.getUserByIdentifier(ctx, arg)
}

func (f *fakeRepo) InsertSession(ctx context.Context, arg InsertSessionParams) (InsertSessionRow, error) {
	return f.insertSession(ctx, arg)
}

func (f *fakeRepo) DeleteSessionByID(ctx context.Context, id uuid.UUID) error {
	return f.deleteSessionByID(ctx, id)
}

func (f *fakeRepo) DeleteSessionsByUser(ctx context.Context, userID uuid.UUID) error {
	return f.deleteSessionsByUser(ctx, userID)
}

func (f *fakeRepo) UpdateUserPassword(ctx context.Context, arg UpdateUserPasswordParams) error {
	return f.updateUserPassword(ctx, arg)
}

func (f *fakeRepo) UpdateUserEmail(ctx context.Context, arg UpdateUserEmailParams) error {
	return f.updateUserEmail(ctx, arg)
}

func (f *fakeRepo) InsertPasswordResetToken(ctx context.Context, arg InsertPasswordResetTokenParams) (PasswordResetToken, error) {
	return f.insertPasswordResetToken(ctx, arg)
}

func (f *fakeRepo) GetPasswordResetTokenByHash(ctx context.Context, tokenHash string) (GetPasswordResetTokenByHashRow, error) {
	return f.getPasswordResetTokenByHash(ctx, tokenHash)
}

func (f *fakeRepo) DeletePasswordResetToken(ctx context.Context, id uuid.UUID) error {
	return f.deletePasswordResetToken(ctx, id)
}

func (f *fakeRepo) CleanupExpiredPasswordResetTokens(ctx context.Context) error {
	return f.cleanupExpiredPasswordResetTokens(ctx)
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
