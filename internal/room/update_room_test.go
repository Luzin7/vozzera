package room

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Luzin7/vozzera-backend/internal/shared/httpx"
	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestUpdateRoomService_Execute(t *testing.T) {
	mod := httpx.UserClaims{UserID: uuid.New(), Role: httpx.RoleAdmin}
	user := httpx.UserClaims{UserID: uuid.New(), Role: httpx.RoleUser}

	t.Run("sem permissão", func(t *testing.T) {
		svc := NewUpdateRoomService(newFakeRepo(), &fakePublisher{}, nil)

		_, err := svc.Execute(context.Background(), UpdateRoomInput{ID: uuid.New(), Name: "sala", Claims: user})
		if !errors.Is(err, ErrNotAuthorized) {
			t.Errorf("erro = %v, want ErrNotAuthorized", err)
		}
	})

	t.Run("nome em branco", func(t *testing.T) {
		svc := NewUpdateRoomService(newFakeRepo(), &fakePublisher{}, nil)

		_, err := svc.Execute(context.Background(), UpdateRoomInput{ID: uuid.New(), Name: "", Claims: mod})
		if !errors.Is(err, ErrNameRequired) {
			t.Errorf("erro = %v, want ErrNameRequired", err)
		}
	})

	t.Run("nome muito longo", func(t *testing.T) {
		svc := NewUpdateRoomService(newFakeRepo(), &fakePublisher{}, nil)

		_, err := svc.Execute(context.Background(), UpdateRoomInput{ID: uuid.New(), Name: strings.Repeat("a", 101), Claims: mod})
		if !errors.Is(err, ErrNameTooLong) {
			t.Errorf("erro = %v, want ErrNameTooLong", err)
		}
	})

	t.Run("sala inexistente", func(t *testing.T) {
		repo := newFakeRepo()
		repo.updateRoom = func(context.Context, UpdateRoomParams) (Room, error) {
			return Room{}, pgx.ErrNoRows
		}
		svc := NewUpdateRoomService(repo, &fakePublisher{}, nil)

		_, err := svc.Execute(context.Background(), UpdateRoomInput{ID: uuid.New(), Name: "sala", Claims: mod})
		if !errors.Is(err, ErrRoomNotFound) {
			t.Errorf("erro = %v, want ErrRoomNotFound", err)
		}
	})

	t.Run("sucesso emite broadcast", func(t *testing.T) {
		roomID := uuid.New()
		repo := newFakeRepo()
		repo.updateRoom = func(_ context.Context, arg UpdateRoomParams) (Room, error) {
			return Room{ID: roomID, Name: arg.Name}, nil
		}
		events := &fakePublisher{}
		svc := NewUpdateRoomService(repo, events, nil)

		out, err := svc.Execute(context.Background(), UpdateRoomInput{ID: roomID, Name: "sala atualizada", Claims: mod})
		if err != nil {
			t.Fatalf("Execute() erro inesperado: %v", err)
		}
		if out.Room.Name != "sala atualizada" {
			t.Errorf("name = %q, want %q", out.Room.Name, "sala atualizada")
		}

		if len(events.envelopes) != 1 {
			t.Fatalf("envelopes = %d, want 1", len(events.envelopes))
		}
		if events.envelopes[0].Type != EventRoomUpdated {
			t.Errorf("type = %q, want %q", events.envelopes[0].Type, EventRoomUpdated)
		}
	})

	t.Run("staff_only revoga assinaturas", func(t *testing.T) {
		roomID := uuid.New()
		repo := newFakeRepo()
		repo.updateRoom = func(context.Context, UpdateRoomParams) (Room, error) {
			return Room{ID: roomID, StaffOnly: true}, nil
		}
		revoker := &fakeRevoker{}
		svc := NewUpdateRoomService(repo, &fakePublisher{}, revoker)

		staffOnly := true
		_, err := svc.Execute(context.Background(), UpdateRoomInput{ID: roomID, Name: "sala", StaffOnly: &staffOnly, Claims: mod})
		if err != nil {
			t.Fatalf("Execute() erro inesperado: %v", err)
		}

		if len(revoker.topics) != 1 {
			t.Fatalf("revoked = %d, want 1", len(revoker.topics))
		}
		want := realtime.Topic("room:" + roomID.String())
		if revoker.topics[0] != want {
			t.Errorf("topic = %q, want %q", revoker.topics[0], want)
		}
	})
}
