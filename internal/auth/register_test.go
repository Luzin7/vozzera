package auth

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

func TestRegisterService_Execute_Validation(t *testing.T) {
	svc := NewRegisterService(newFakeRepo(), "invite")

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

func TestRegisterService_Execute_Success(t *testing.T) {
	repo := newFakeRepo()
	var got CreateUserParams
	repo.createUser = func(ctx context.Context, arg CreateUserParams) (CreateUserRow, error) {
		got = arg
		return CreateUserRow{ID: uuid.New(), Username: arg.Username}, nil
	}

	svc := NewRegisterService(repo, "invite")

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
}
