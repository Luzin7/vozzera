package auth

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/Luzin7/vozzera-backend/internal/shared/httpx"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestRegisterService_Execute_Validation(t *testing.T) {
	svc := NewRegisterService(newFakeRepo(), "invite", &userObserverRecorder{})

	tests := []struct {
		name   string
		input  RegisterInput
		status int
	}{
		{
			name:   "código de convite inválido",
			input:  RegisterInput{Username: "luand", Password: "secret123", Email: "luan@example.com", InviteCode: "x"},
			status: http.StatusForbidden,
		},
		{
			name:   "username curto",
			input:  RegisterInput{Username: "ab", Password: "secret123", Email: "luan@example.com", InviteCode: "invite"},
			status: http.StatusBadRequest,
		},
		{
			name:   "username longo",
			input:  RegisterInput{Username: "a", Password: "secret123", Email: "luan@example.com", InviteCode: "invite"},
			status: http.StatusBadRequest,
		},
		{
			name:   "senha curta",
			input:  RegisterInput{Username: "luand", Password: "short", Email: "luan@example.com", InviteCode: "invite"},
			status: http.StatusBadRequest,
		},
		{
			name:   "email inválido",
			input:  RegisterInput{Username: "luand", Password: "secret123", Email: "sem-arroba", InviteCode: "invite"},
			status: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Execute(context.Background(), tt.input)
			assertStatus(t, err, tt.status)
		})
	}
}

type userObserverRecorder struct {
	created int
}

func (r *userObserverRecorder) UserCreated(context.Context) { r.created++ }

func TestRegisterService_Execute_NotifiesObserverOnlyOnSuccess(t *testing.T) {
	repo := newFakeRepo()
	repo.createUser = func(context.Context, CreateUserParams) (CreateUserRow, error) {
		return CreateUserRow{}, errors.New("falha")
	}
	observer := &userObserverRecorder{}

	_, err := NewRegisterService(repo, "invite", observer).Execute(context.Background(), RegisterInput{
		Username: "luand", Password: "secret123", Email: "luan@example.com", InviteCode: "invite",
	})

	if err == nil {
		t.Fatal("Execute() deveria falhar")
	}
	if observer.created != 0 {
		t.Errorf("created = %d, want 0", observer.created)
	}
}

func TestRegisterService_Execute_Success(t *testing.T) {
	repo := newFakeRepo()
	var got CreateUserParams
	repo.createUser = func(ctx context.Context, arg CreateUserParams) (CreateUserRow, error) {
		got = arg
		return CreateUserRow{ID: uuid.New(), Username: arg.Username}, nil
	}

	observer := &userObserverRecorder{}
	svc := NewRegisterService(repo, "invite", observer)

	out, err := svc.Execute(context.Background(), RegisterInput{
		Username:   "  luand  ",
		Password:   "secret123",
		Email:      "  Luan@Example.com  ",
		InviteCode: "invite",
	})
	if err != nil {
		t.Fatalf("Execute() erro inesperado: %v", err)
	}

	if got.Username != "luand" {
		t.Errorf("username no repositório = %q, want %q", got.Username, "luand")
	}
	if got.Email != "luan@example.com" {
		t.Errorf("email no repositório = %q, want %q", got.Email, "luan@example.com")
	}
	if got.PasswordHash == "" {
		t.Error("senha não foi hashada")
	}
	if out.Username != "luand" {
		t.Errorf("out.Username = %q, want %q", out.Username, "luand")
	}
	if observer.created != 1 {
		t.Errorf("created = %d, want 1", observer.created)
	}
}

func TestRegisterService_Execute_CreateUserFailure(t *testing.T) {
	tests := []struct {
		name    string
		repoErr error
		status  int
		message string
	}{
		{
			name:    "unique no username",
			repoErr: &pgconn.PgError{Code: "23505", ConstraintName: "users_username_key"},
			status:  http.StatusConflict,
			message: ErrUsernameTaken(nil).(*httpx.Error).Message,
		},
		{
			name:    "unique no email",
			repoErr: &pgconn.PgError{Code: "23505", ConstraintName: "users_email_key"},
			status:  http.StatusConflict,
			message: ErrEmailTaken().(*httpx.Error).Message,
		},
		{
			name:    "unique em outra constraint",
			repoErr: &pgconn.PgError{Code: "23505", ConstraintName: "users_pkey"},
			status:  http.StatusInternalServerError,
			message: ErrCreateUser(nil).(*httpx.Error).Message,
		},
		{
			name:    "falha de infraestrutura",
			repoErr: errors.New("conexão recusada"),
			status:  http.StatusInternalServerError,
			message: ErrCreateUser(nil).(*httpx.Error).Message,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakeRepo()
			repo.createUser = func(context.Context, CreateUserParams) (CreateUserRow, error) {
				return CreateUserRow{}, tt.repoErr
			}
			svc := NewRegisterService(repo, "invite", &userObserverRecorder{})

			_, err := svc.Execute(context.Background(), RegisterInput{
				Username: "luand", Password: "secret123", Email: "luan@example.com", InviteCode: "invite",
			})

			assertStatus(t, err, tt.status)
			var e *httpx.Error
			errors.As(err, &e)
			if e.Message != tt.message {
				t.Errorf("message = %q, want %q", e.Message, tt.message)
			}
		})
	}
}
