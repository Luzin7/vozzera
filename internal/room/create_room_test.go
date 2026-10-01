package room

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Luzin7/vozzera-backend/internal/shared/httpx"
	"github.com/google/uuid"
)

func TestCreateRoomService_Execute(t *testing.T) {
	mod := httpx.UserClaims{UserID: uuid.New(), Role: httpx.RoleMod}
	user := httpx.UserClaims{UserID: uuid.New(), Role: httpx.RoleUser}

	t.Run("sem permissão", func(t *testing.T) {
		svc := NewCreateRoomService(newFakeRepo(), &fakePublisher{})

		_, err := svc.Execute(context.Background(), CreateRoomInput{Name: "sala", Claims: user})
		if !errors.Is(err, ErrNotAuthorized) {
			t.Errorf("erro = %v, want ErrNotAuthorized", err)
		}
	})

	t.Run("nome em branco", func(t *testing.T) {
		svc := NewCreateRoomService(newFakeRepo(), &fakePublisher{})

		_, err := svc.Execute(context.Background(), CreateRoomInput{Name: "", Claims: mod})
		if !errors.Is(err, ErrNameRequired) {
			t.Errorf("erro = %v, want ErrNameRequired", err)
		}
	})

	t.Run("sucesso emite broadcast", func(t *testing.T) {
		repo := newFakeRepo()
		repo.createRoom = func(_ context.Context, arg CreateRoomParams) (Room, error) {
			return Room{ID: uuid.New(), Name: arg.Name, HasVoice: arg.HasVoice}, nil
		}
		events := &fakePublisher{}
		svc := NewCreateRoomService(repo, events)

		out, err := svc.Execute(context.Background(), CreateRoomInput{Name: "sala", HasVoice: true, Claims: mod})
		if err != nil {
			t.Fatalf("Execute() erro inesperado: %v", err)
		}
		if out.Room.Name != "sala" {
			t.Errorf("name = %q, want %q", out.Room.Name, "sala")
		}

		if len(events.envelopes) != 1 {
			t.Fatalf("envelopes = %d, want 1", len(events.envelopes))
		}
		env := events.envelopes[0]
		if env.Type != EventRoomCreated {
			t.Errorf("type = %q, want %q", env.Type, EventRoomCreated)
		}

		var payload RoomPayload
		if err := json.Unmarshal(env.Data, &payload); err != nil {
			t.Fatalf("Unmarshal payload: %v", err)
		}
		if !payload.HasVoice {
			t.Error("payload.HasVoice = false, want true")
		}
	})

	t.Run("staff_only na criação", func(t *testing.T) {
		repo := newFakeRepo()
		repo.createRoom = func(_ context.Context, arg CreateRoomParams) (Room, error) {
			return Room{ID: uuid.New(), Name: arg.Name, StaffOnly: arg.StaffOnly}, nil
		}
		events := &fakePublisher{}
		svc := NewCreateRoomService(repo, events)

		out, err := svc.Execute(context.Background(), CreateRoomInput{Name: "sala", StaffOnly: true, Claims: mod})
		if err != nil {
			t.Fatalf("Execute() erro inesperado: %v", err)
		}
		if !out.Room.StaffOnly {
			t.Error("out.Room.StaffOnly = false, want true")
		}

		var payload RoomPayload
		if err := json.Unmarshal(events.envelopes[0].Data, &payload); err != nil {
			t.Fatalf("Unmarshal payload: %v", err)
		}
		if !payload.StaffOnly {
			t.Error("payload.StaffOnly = false, want true")
		}
	})
}
