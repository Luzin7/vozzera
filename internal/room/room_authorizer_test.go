package room

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/Luzin7/vozzera-backend/internal/shared/httpx"
	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestRoomAuthorizer_CanSubscribe(t *testing.T) {
	userID := uuid.New()

	t.Run("sala pública", func(t *testing.T) {
		roomID := uuid.New()
		repo := newFakeRepo()
		repo.getRoomByID = func(_ context.Context, id uuid.UUID) (Room, error) {
			return Room{ID: id}, nil
		}

		auth := NewRoomAuthorizer(repo)
		err := auth.CanSubscribe(context.Background(), userID.String(), realtime.Topic("room:"+roomID.String()))
		if err != nil {
			t.Errorf("erro = %v, want nil", err)
		}
	})

	t.Run("staff_only com moderador", func(t *testing.T) {
		roomID := uuid.New()
		repo := newFakeRepo()
		repo.getRoomByID = func(_ context.Context, id uuid.UUID) (Room, error) {
			return Room{ID: id, StaffOnly: true}, nil
		}
		repo.getUserRole = func(context.Context, uuid.UUID) (string, error) {
			return httpx.RoleMod, nil
		}

		auth := NewRoomAuthorizer(repo)
		err := auth.CanSubscribe(context.Background(), userID.String(), realtime.Topic("room:"+roomID.String()))
		if err != nil {
			t.Errorf("erro = %v, want nil", err)
		}
	})

	t.Run("staff_only com usuário comum", func(t *testing.T) {
		roomID := uuid.New()
		repo := newFakeRepo()
		repo.getRoomByID = func(_ context.Context, id uuid.UUID) (Room, error) {
			return Room{ID: id, StaffOnly: true}, nil
		}
		repo.getUserRole = func(context.Context, uuid.UUID) (string, error) {
			return httpx.RoleUser, nil
		}

		auth := NewRoomAuthorizer(repo)
		err := auth.CanSubscribe(context.Background(), userID.String(), realtime.Topic("room:"+roomID.String()))
		if !errors.Is(err, ErrRoomNotFound) {
			t.Errorf("erro = %v, want ErrRoomNotFound", err)
		}
	})

	t.Run("sala inexistente", func(t *testing.T) {
		repo := newFakeRepo()
		repo.getRoomByID = func(context.Context, uuid.UUID) (Room, error) {
			return Room{}, pgx.ErrNoRows
		}

		auth := NewRoomAuthorizer(repo)
		err := auth.CanSubscribe(context.Background(), userID.String(), realtime.Topic("room:"+uuid.New().String()))
		if !errors.Is(err, ErrRoomNotFound) {
			t.Errorf("erro = %v, want ErrRoomNotFound", err)
		}
	})

	t.Run("tópico inválido", func(t *testing.T) {
		auth := NewRoomAuthorizer(newFakeRepo())
		err := auth.CanSubscribe(context.Background(), userID.String(), realtime.Topic("invalid"))
		if !errors.Is(err, ErrRoomNotFound) {
			t.Errorf("erro = %v, want ErrRoomNotFound", err)
		}
	})
}

func TestRoomAuthorizer_CanSubscribe_DatabaseFailureIsInternalError(t *testing.T) {
	topic := realtime.RoomTopic(uuid.New())

	t.Run("falha no banco ao buscar a sala", func(t *testing.T) {
		repo := newFakeRepo()
		repo.getRoomByID = func(context.Context, uuid.UUID) (Room, error) { return Room{}, errors.New("conexão caiu") }

		err := NewRoomAuthorizer(repo).CanSubscribe(context.Background(), uuid.NewString(), topic)
		assertInternalError(t, err)
	})

	t.Run("falha no banco ao buscar o papel", func(t *testing.T) {
		repo := newFakeRepo()
		repo.getRoomByID = func(_ context.Context, id uuid.UUID) (Room, error) { return Room{ID: id, StaffOnly: true}, nil }
		repo.getUserRole = func(context.Context, uuid.UUID) (string, error) { return "", errors.New("conexão caiu") }

		err := NewRoomAuthorizer(repo).CanSubscribe(context.Background(), uuid.NewString(), topic)
		assertInternalError(t, err)
	})
}

func assertInternalError(t *testing.T, err error) {
	t.Helper()
	var httpErr *httpx.Error
	if !errors.As(err, &httpErr) || httpErr.Status != http.StatusInternalServerError {
		t.Errorf("erro = %v, want erro interno 500", err)
	}
}
