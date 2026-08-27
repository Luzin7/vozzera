package voice

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/Luzin7/vozzera-backend/internal/shared/httpx"
)

func TestListVoiceRoomsService_Execute(t *testing.T) {
	repo := newFakeRepo()
	svc := NewListVoiceRoomsService(repo)

	t.Run("sucesso", func(t *testing.T) {
		id := uuid.New()
		now := pgtype.Timestamptz{Valid: true}
		rooms := []Room{
			{ID: id, Name: "Sala Geral", Type: "voice", CreatedAt: now, UpdatedAt: now},
		}
		repo.listVoiceRooms = func(context.Context) ([]Room, error) {
			return rooms, nil
		}

		out, err := svc.Execute(context.Background())
		if err != nil {
			t.Fatalf("Execute() erro inesperado: %v", err)
		}
		if len(out.Rooms) != 1 {
			t.Fatalf("esperava 1 sala, obteve %d", len(out.Rooms))
		}
		if out.Rooms[0].ID != id {
			t.Errorf("room id = %v, want %v", out.Rooms[0].ID, id)
		}
		if out.Rooms[0].Name != "Sala Geral" {
			t.Errorf("room name = %q, want %q", out.Rooms[0].Name, "Sala Geral")
		}
	})

	t.Run("sem salas", func(t *testing.T) {
		repo.listVoiceRooms = func(context.Context) ([]Room, error) {
			return nil, nil
		}

		out, err := svc.Execute(context.Background())
		if err != nil {
			t.Fatalf("Execute() erro inesperado: %v", err)
		}
		if out.Rooms == nil {
			t.Error("out.Rooms é nil, esperava slice vazio")
		}
		if len(out.Rooms) != 0 {
			t.Errorf("len(out.Rooms) = %d, want 0", len(out.Rooms))
		}
	})

	t.Run("erro do repositório", func(t *testing.T) {
		repo.listVoiceRooms = func(context.Context) ([]Room, error) {
			return nil, errors.New("erro de banco")
		}

		_, err := svc.Execute(context.Background())
		if err == nil {
			t.Fatal("Execute() esperava erro, obteve nil")
		}
		var httpxErr *httpx.Error
		if !errors.As(err, &httpxErr) {
			t.Fatalf("erro não é do tipo *httpx.Error: %T", err)
		}
		if httpxErr.Status != 500 {
			t.Errorf("status = %d, want 500", httpxErr.Status)
		}
	})
}