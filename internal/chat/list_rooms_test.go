package chat

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestListRoomsService_Execute(t *testing.T) {
	t.Run("sucesso", func(t *testing.T) {
		repo := newFakeRepo()
		room1 := Room{ID: uuid.New(), Name: "sala a", Type: "text"}
		room2 := Room{ID: uuid.New(), Name: "sala b", Type: "voice"}
		repo.listRooms = func(ctx context.Context) ([]Room, error) {
			return []Room{room1, room2}, nil
		}
		svc := NewListRoomsService(repo)

		out, err := svc.Execute(context.Background())
		if err != nil {
			t.Fatalf("Execute() erro inesperado: %v", err)
		}
		if len(out.Rooms) != 2 {
			t.Fatalf("len(Rooms) = %d, want 2", len(out.Rooms))
		}
		if out.Rooms[0].ID != room1.ID {
			t.Errorf("Rooms[0].id = %v, want %v", out.Rooms[0].ID, room1.ID)
		}
		if out.Rooms[1].ID != room2.ID {
			t.Errorf("Rooms[1].id = %v, want %v", out.Rooms[1].ID, room2.ID)
		}
	})

	t.Run("sem salas", func(t *testing.T) {
		repo := newFakeRepo()
		repo.listRooms = func(ctx context.Context) ([]Room, error) {
			return nil, nil
		}
		svc := NewListRoomsService(repo)

		out, err := svc.Execute(context.Background())
		if err != nil {
			t.Fatalf("Execute() erro inesperado: %v", err)
		}
		if out.Rooms == nil {
			t.Error("Rooms = nil, want empty slice")
		}
		if len(out.Rooms) != 0 {
			t.Errorf("len(Rooms) = %d, want 0", len(out.Rooms))
		}
	})

	t.Run("erro do repositorio", func(t *testing.T) {
		repo := newFakeRepo()
		sentinelErr := errors.New("db error")
		repo.listRooms = func(ctx context.Context) ([]Room, error) {
			return nil, sentinelErr
		}
		svc := NewListRoomsService(repo)

		_, err := svc.Execute(context.Background())
		if err == nil {
			t.Fatal("expected error")
		}
	})
}