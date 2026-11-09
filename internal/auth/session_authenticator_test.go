package auth

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestAuthenticateSession_ExpiredSessionIsInvalid(t *testing.T) {
	repo := newFakeRepo()
	repo.getSessionByID = func(context.Context, uuid.UUID) (GetSessionByIDRow, error) {
		return GetSessionByIDRow{ExpiresAt: time.Now().Add(-time.Minute)}, nil
	}

	_, err := NewSessionAuthenticator(repo, time.Hour, time.Hour).AuthenticateSession(context.Background(), uuid.NewString())

	if !IsSessionInvalid(err) {
		t.Fatalf("erro = %v, want sessão inválida", err)
	}
	assertStatus(t, err, http.StatusUnauthorized)
}

func TestAuthenticateSession_MissingSessionIsInvalid(t *testing.T) {
	repo := newFakeRepo()
	repo.getSessionByID = func(context.Context, uuid.UUID) (GetSessionByIDRow, error) {
		return GetSessionByIDRow{}, pgx.ErrNoRows
	}

	_, err := NewSessionAuthenticator(repo, time.Hour, time.Hour).AuthenticateSession(context.Background(), uuid.NewString())

	if !IsSessionInvalid(err) {
		t.Fatalf("erro = %v, want sessão inválida", err)
	}
	assertStatus(t, err, http.StatusUnauthorized)
}

func TestAuthenticateSession_DatabaseErrorIsNotInvalidSession(t *testing.T) {
	repo := newFakeRepo()
	repo.getSessionByID = func(context.Context, uuid.UUID) (GetSessionByIDRow, error) {
		return GetSessionByIDRow{}, errors.New("banco indisponível")
	}

	_, err := NewSessionAuthenticator(repo, time.Hour, time.Hour).AuthenticateSession(context.Background(), uuid.NewString())

	if IsSessionInvalid(err) {
		t.Fatalf("erro = %v, não deveria ser classificada como sessão inválida", err)
	}
}

func TestAuthenticateSession_ValidSessionReturnsClaims(t *testing.T) {
	repo := newFakeRepo()
	userID := uuid.New()
	expiresAt := time.Now().Add(2 * time.Hour)
	repo.getSessionByID = func(context.Context, uuid.UUID) (GetSessionByIDRow, error) {
		return GetSessionByIDRow{
			ID:        uuid.New(),
			UserID:    userID,
			Username:  "alice",
			Role:      "user",
			ExpiresAt: expiresAt,
		}, nil
	}

	claims, err := NewSessionAuthenticator(repo, time.Hour, time.Hour).AuthenticateSession(context.Background(), uuid.NewString())

	if err != nil {
		t.Fatalf("AuthenticateSession erro: %v", err)
	}
	if claims.UserID != userID || claims.Username != "alice" || !claims.ExpiresAt.Equal(expiresAt) {
		t.Errorf("claims inesperadas: %+v", claims)
	}
}
