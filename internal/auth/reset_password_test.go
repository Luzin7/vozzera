package auth

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type fakeTransactor struct {
	repo Repository
	runs int
	err  error
}

func (f *fakeTransactor) WithinTx(_ context.Context, fn func(Repository) error) error {
	f.runs++
	if f.err != nil {
		return f.err
	}
	return fn(f.repo)
}

func successfulResetRepo(userID uuid.UUID) *fakeRepo {
	repo := newFakeRepo()
	repo.consumePasswordResetToken = func(context.Context, string) (uuid.UUID, error) { return userID, nil }
	repo.updateUserPassword = func(context.Context, UpdateUserPasswordParams) error { return nil }
	repo.deletePasswordResetTokensByUser = func(context.Context, uuid.UUID) error { return nil }
	repo.deleteSessionsByUser = func(context.Context, uuid.UUID) error { return nil }
	return repo
}

func TestResetPasswordService_Execute_InvalidToken(t *testing.T) {
	svc := NewResetPasswordService(&fakeTransactor{repo: newFakeRepo()}, &revokeRecorder{})

	_, err := svc.Execute(context.Background(), ResetPasswordInput{Token: "", Password: "secret123"})
	assertStatus(t, err, http.StatusBadRequest)
}

func TestResetPasswordService_Execute_InvalidPassword(t *testing.T) {
	svc := NewResetPasswordService(&fakeTransactor{repo: newFakeRepo()}, &revokeRecorder{})

	_, err := svc.Execute(context.Background(), ResetPasswordInput{Token: "abc", Password: "short"})
	assertStatus(t, err, http.StatusBadRequest)
}

func TestResetPasswordService_Execute_TokenNotFound(t *testing.T) {
	repo := newFakeRepo()
	repo.consumePasswordResetToken = func(context.Context, string) (uuid.UUID, error) {
		return uuid.Nil, pgx.ErrNoRows
	}
	revoker := &revokeRecorder{}
	svc := NewResetPasswordService(&fakeTransactor{repo: repo}, revoker)

	_, err := svc.Execute(context.Background(), ResetPasswordInput{Token: "abc", Password: "secret123"})
	assertStatus(t, err, http.StatusBadRequest)

	if revoker.revokedUser != uuid.Nil {
		t.Errorf("websockets revogados de %v com token inválido", revoker.revokedUser)
	}
}

func TestResetPasswordService_Execute_Success(t *testing.T) {
	userID := uuid.New()
	repo := successfulResetRepo(userID)

	var consumedHash string
	repo.consumePasswordResetToken = func(_ context.Context, tokenHash string) (uuid.UUID, error) {
		consumedHash = tokenHash
		return userID, nil
	}

	var updated UpdateUserPasswordParams
	repo.updateUserPassword = func(_ context.Context, arg UpdateUserPasswordParams) error {
		updated = arg
		return nil
	}

	var tokensDeletedFor uuid.UUID
	repo.deletePasswordResetTokensByUser = func(_ context.Context, id uuid.UUID) error {
		tokensDeletedFor = id
		return nil
	}

	var sessionsDeletedFor uuid.UUID
	repo.deleteSessionsByUser = func(_ context.Context, id uuid.UUID) error {
		sessionsDeletedFor = id
		return nil
	}

	transactor := &fakeTransactor{repo: repo}
	revoker := &revokeRecorder{}
	svc := NewResetPasswordService(transactor, revoker)

	_, err := svc.Execute(context.Background(), ResetPasswordInput{Token: "abc123", Password: "newsecret123"})
	if err != nil {
		t.Fatalf("Execute() erro inesperado: %v", err)
	}

	if transactor.runs != 1 {
		t.Errorf("transações = %d, want 1", transactor.runs)
	}
	if consumedHash != hashResetToken("abc123") {
		t.Errorf("token consumido = %q, want o hash do token recebido", consumedHash)
	}
	if updated.ID != userID {
		t.Errorf("senha atualizada para %v, want %v", updated.ID, userID)
	}
	if err := CheckPassword(updated.PasswordHash, "newsecret123"); err != nil {
		t.Error("senha não foi hashada corretamente")
	}
	if tokensDeletedFor != userID {
		t.Errorf("tokens apagados de %v, want %v", tokensDeletedFor, userID)
	}
	if sessionsDeletedFor != userID {
		t.Errorf("sessões apagadas de %v, want %v", sessionsDeletedFor, userID)
	}
	if revoker.revokedUser != userID {
		t.Errorf("websockets revogados de %v, want %v", revoker.revokedUser, userID)
	}
}

func TestResetPasswordService_Execute_FailureDoesNotRevokeSockets(t *testing.T) {
	repo := successfulResetRepo(uuid.New())
	repo.deleteSessionsByUser = func(context.Context, uuid.UUID) error {
		return errors.New("falha ao apagar sessões")
	}
	revoker := &revokeRecorder{}
	svc := NewResetPasswordService(&fakeTransactor{repo: repo}, revoker)

	_, err := svc.Execute(context.Background(), ResetPasswordInput{Token: "abc123", Password: "newsecret123"})
	assertStatus(t, err, http.StatusInternalServerError)

	if revoker.revokedUser != uuid.Nil {
		t.Errorf("websockets revogados de %v numa redefinição que falhou", revoker.revokedUser)
	}
}

func TestResetPasswordService_Execute_TransactionFailureIsNamedError(t *testing.T) {
	transactor := &fakeTransactor{repo: successfulResetRepo(uuid.New()), err: errors.New("commit: conn closed")}
	svc := NewResetPasswordService(transactor, &revokeRecorder{})

	_, err := svc.Execute(context.Background(), ResetPasswordInput{Token: "abc123", Password: "newsecret123"})
	assertStatus(t, err, http.StatusInternalServerError)
}

func TestResetPasswordService_Execute_RevokeFailureStillSucceeds(t *testing.T) {
	svc := NewResetPasswordService(
		&fakeTransactor{repo: successfulResetRepo(uuid.New())},
		&revokeRecorder{err: errors.New("hub encerrado")},
	)

	_, err := svc.Execute(context.Background(), ResetPasswordInput{Token: "abc123", Password: "newsecret123"})
	if err != nil {
		t.Fatalf("Execute() erro inesperado: %v", err)
	}
}

func TestResetPasswordService_Execute_RevokesAfterRequestIsCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	revoker := &revokeRecorder{}
	svc := NewResetPasswordService(&fakeTransactor{repo: successfulResetRepo(uuid.New())}, revoker)

	_, _ = svc.Execute(ctx, ResetPasswordInput{Token: "abc123", Password: "newsecret123"})

	if revoker.ctxErr != nil {
		t.Errorf("revogação rodou com o contexto da requisição cancelado: %v", revoker.ctxErr)
	}
}
