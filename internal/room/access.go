package room

import (
	"context"
	"errors"

	"github.com/Luzin7/vozzera-backend/internal/shared/httpx"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Access struct {
	repo Repository
}

func NewAccess(repo Repository) *Access {
	return &Access{repo: repo}
}

func (a *Access) CanAccess(ctx context.Context, roomID uuid.UUID, claims httpx.UserClaims) (bool, error) {
	room, err := a.repo.GetRoomByID(ctx, roomID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return canAccess(room, claims), nil
}

func (a *Access) AuthorizeVoice(ctx context.Context, roomID uuid.UUID, claims httpx.UserClaims) (string, error) {
	room, err := a.repo.GetRoomByID(ctx, roomID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrRoomNotFound
	}
	if err != nil {
		return "", ErrGetRoom(err)
	}
	if !canAccess(room, claims) {
		return "", ErrRoomNotFound
	}
	if !room.HasVoice {
		return "", ErrNotVoiceRoom
	}
	return room.Name, nil
}

func canAccess(room Room, claims httpx.UserClaims) bool {
	if claims.CanModerate() {
		return true
	}
	return !room.StaffOnly
}
