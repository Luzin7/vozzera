package voice

import (
	"context"
	"errors"
	"testing"

	"github.com/Luzin7/vozzera-backend/internal/shared/httpx"
	"github.com/google/uuid"
)

type fakeAccess struct {
	authorizeVoice func(context.Context, uuid.UUID, httpx.UserClaims) (string, error)
}

func (f *fakeAccess) AuthorizeVoice(ctx context.Context, roomID uuid.UUID, claims httpx.UserClaims) (string, error) {
	return f.authorizeVoice(ctx, roomID, claims)
}

var _ VoiceRoomAccess = (*fakeAccess)(nil)

func TestTokenService_Execute(t *testing.T) {
	claims := httpx.UserClaims{UserID: uuid.New(), Username: "luand", Role: httpx.RoleUser}

	t.Run("sala não é de voz", func(t *testing.T) {
		access := &fakeAccess{authorizeVoice: func(context.Context, uuid.UUID, httpx.UserClaims) (string, error) {
			return "", httpx.Errorf(400, "Esta sala não é de voz")
		}}
		svc := NewTokenService(access, NewTokenIssuer("key", "secret"), "wss://livekit")

		_, err := svc.Execute(context.Background(), TokenInput{Claims: claims, RoomID: uuid.New()})
		assertStatus(t, err, 400)
	})

	t.Run("sala inexistente", func(t *testing.T) {
		access := &fakeAccess{authorizeVoice: func(context.Context, uuid.UUID, httpx.UserClaims) (string, error) {
			return "", httpx.Errorf(404, "Sala não encontrada")
		}}
		svc := NewTokenService(access, NewTokenIssuer("key", "secret"), "wss://livekit")

		_, err := svc.Execute(context.Background(), TokenInput{Claims: claims, RoomID: uuid.New()})
		assertStatus(t, err, 404)
	})

	t.Run("sucesso", func(t *testing.T) {
		access := &fakeAccess{authorizeVoice: func(context.Context, uuid.UUID, httpx.UserClaims) (string, error) {
			return "voz", nil
		}}
		svc := NewTokenService(access, NewTokenIssuer("key", "secret"), "wss://livekit")

		out, err := svc.Execute(context.Background(), TokenInput{Claims: claims, RoomID: uuid.New()})
		if err != nil {
			t.Fatalf("Execute() erro inesperado: %v", err)
		}
		if out.Token == "" {
			t.Error("token vazio")
		}
		if out.URL != "wss://livekit" {
			t.Errorf("url = %q, want %q", out.URL, "wss://livekit")
		}
		if out.RoomName != "voz" {
			t.Errorf("room name = %q, want %q", out.RoomName, "voz")
		}
	})
}

func assertStatus(t *testing.T, err error, status int) {
	t.Helper()

	if err == nil {
		t.Fatal("esperava erro, obteve nil")
	}

	var httpErr *httpx.Error
	if !errors.As(err, &httpErr) {
		t.Fatalf("erro não é do tipo *httpx.Error: %T", err)
	}
	if httpErr.Status != status {
		t.Errorf("status = %d, want %d", httpErr.Status, status)
	}
}
