package chat

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestUpdateRoomService_Execute(t *testing.T) {
	t.Run("sem permissao", func(t *testing.T) {
		repo := newFakeRepo()
		events := &fakePublisher{}
		svc := NewUpdateRoomService(repo, events)

		_, err := svc.Execute(context.Background(), UpdateRoomInput{ID: uuid.New(), Name: "sala", IsMod: false})
		if !errors.Is(err, ErrNotAuthorized) {
			t.Errorf("erro = %v, want ErrNotAuthorized", err)
		}
	})

	t.Run("nome em branco", func(t *testing.T) {
		repo := newFakeRepo()
		events := &fakePublisher{}
		svc := NewUpdateRoomService(repo, events)

		_, err := svc.Execute(context.Background(), UpdateRoomInput{ID: uuid.New(), Name: "", IsMod: true})
		if !errors.Is(err, ErrNameRequired) {
			t.Errorf("erro = %v, want ErrNameRequired", err)
		}
	})

	t.Run("nome muito longo", func(t *testing.T) {
		repo := newFakeRepo()
		events := &fakePublisher{}
		svc := NewUpdateRoomService(repo, events)

		_, err := svc.Execute(context.Background(), UpdateRoomInput{ID: uuid.New(), Name: strings.Repeat("a", 101), IsMod: true})
		if !errors.Is(err, ErrNameTooLong) {
			t.Errorf("erro = %v, want ErrNameTooLong", err)
		}
	})

	t.Run("sala inexistente", func(t *testing.T) {
		repo := newFakeRepo()
		repo.updateRoom = func(ctx context.Context, arg UpdateRoomParams) (Room, error) {
			return Room{}, pgx.ErrNoRows
		}
		events := &fakePublisher{}
		svc := NewUpdateRoomService(repo, events)

		_, err := svc.Execute(context.Background(), UpdateRoomInput{ID: uuid.New(), Name: "sala", IsMod: true})
		if !errors.Is(err, ErrRoomNotFound) {
			t.Errorf("erro = %v, want ErrRoomNotFound", err)
		}
	})

	t.Run("sucesso emite broadcast", func(t *testing.T) {
		roomID := uuid.New()
		repo := newFakeRepo()
		repo.updateRoom = func(ctx context.Context, arg UpdateRoomParams) (Room, error) {
			return Room{ID: roomID, Name: arg.Name, Type: "text"}, nil
		}
		events := &fakePublisher{}
		svc := NewUpdateRoomService(repo, events)

		out, err := svc.Execute(context.Background(), UpdateRoomInput{ID: roomID, Name: "sala atualizada", IsMod: true})
		if err != nil {
			t.Fatalf("Execute() erro inesperado: %v", err)
		}
		if out.Room.Name != "sala atualizada" {
			t.Errorf("name = %q, want %q", out.Room.Name, "sala atualizada")
		}

		if len(events.envelopes) != 1 {
			t.Fatalf("envelopes = %d, want 1", len(events.envelopes))
		}
		env := events.envelopes[0]
		if env.Type != EventRoomUpdated {
			t.Errorf("type = %q, want %q", env.Type, EventRoomUpdated)
		}
		var payload RoomPayload
		if err := json.Unmarshal(env.Data, &payload); err != nil {
			t.Fatalf("Unmarshal payload: %v", err)
		}
		if payload.Name != "sala atualizada" {
			t.Errorf("name = %q, want %q", payload.Name, "sala atualizada")
		}
	})
}