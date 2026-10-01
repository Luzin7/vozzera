package room

import (
	"context"
	"errors"
	"testing"

	"github.com/Luzin7/vozzera-backend/internal/shared/httpx"
	"github.com/google/uuid"
)

func TestListRoomsService_Execute(t *testing.T) {
	user := httpx.UserClaims{UserID: uuid.New(), Role: httpx.RoleUser}
	mod := httpx.UserClaims{UserID: uuid.New(), Role: httpx.RoleAdmin}

	publicRoom := Room{ID: uuid.New(), Name: "pública"}
	restrictedRoom := Room{ID: uuid.New(), Name: "restrita", StaffOnly: true}
	voiceRoom := Room{ID: uuid.New(), Name: "voz", HasVoice: true}

	t.Run("usuário comum não vê staff_only", func(t *testing.T) {
		repo := newFakeRepo()
		repo.listRooms = func(context.Context) ([]Room, error) {
			return []Room{publicRoom, restrictedRoom}, nil
		}
		svc := NewListRoomsService(repo)

		out, err := svc.Execute(context.Background(), user, RoomFilter{})
		if err != nil {
			t.Fatalf("Execute() erro inesperado: %v", err)
		}
		if len(out.Rooms) != 1 || out.Rooms[0].ID != publicRoom.ID {
			t.Fatalf("Rooms = %+v, want apenas pública", out.Rooms)
		}
	})

	t.Run("moderador vê staff_only", func(t *testing.T) {
		repo := newFakeRepo()
		repo.listRooms = func(context.Context) ([]Room, error) {
			return []Room{publicRoom, restrictedRoom}, nil
		}
		svc := NewListRoomsService(repo)

		out, err := svc.Execute(context.Background(), mod, RoomFilter{})
		if err != nil {
			t.Fatalf("Execute() erro inesperado: %v", err)
		}
		if len(out.Rooms) != 2 {
			t.Fatalf("len(Rooms) = %d, want 2", len(out.Rooms))
		}
	})

	t.Run("filtro has_voice", func(t *testing.T) {
		repo := newFakeRepo()
		repo.listRooms = func(context.Context) ([]Room, error) {
			return []Room{publicRoom, voiceRoom}, nil
		}
		svc := NewListRoomsService(repo)

		want := true
		out, err := svc.Execute(context.Background(), user, RoomFilter{HasVoice: &want})
		if err != nil {
			t.Fatalf("Execute() erro inesperado: %v", err)
		}
		if len(out.Rooms) != 1 || out.Rooms[0].ID != voiceRoom.ID {
			t.Fatalf("Rooms = %+v, want apenas sala de voz", out.Rooms)
		}
	})

	t.Run("sem salas", func(t *testing.T) {
		repo := newFakeRepo()
		repo.listRooms = func(context.Context) ([]Room, error) {
			return nil, nil
		}
		svc := NewListRoomsService(repo)

		out, err := svc.Execute(context.Background(), user, RoomFilter{})
		if err != nil {
			t.Fatalf("Execute() erro inesperado: %v", err)
		}
		if out.Rooms == nil {
			t.Error("Rooms = nil, want empty slice")
		}
	})

	t.Run("erro do repositório", func(t *testing.T) {
		repo := newFakeRepo()
		repo.listRooms = func(context.Context) ([]Room, error) {
			return nil, errors.New("db error")
		}
		svc := NewListRoomsService(repo)

		if _, err := svc.Execute(context.Background(), user, RoomFilter{}); err == nil {
			t.Fatal("expected error")
		}
	})
}
