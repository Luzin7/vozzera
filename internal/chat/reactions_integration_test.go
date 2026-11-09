package chat

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Luzin7/vozzera-backend/internal/shared/httpx"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestAddReactionService_ConcurrentAtCap(t *testing.T) {
	dbURL := os.Getenv("VOZZERA_TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("VOZZERA_TEST_DATABASE_URL não definido; pulando teste de concorrência real")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("conectar no banco: %v", err)
	}
	defer pool.Close()

	roomID, messageID, userID, err := seedReactionMessage(ctx, pool, maxDistinctEmojis-1)
	if err != nil {
		t.Fatalf("preparar fixture: %v", err)
	}

	repo := New(pool)
	transactor := NewPgTransactor(pool)
	emojis := []string{"🔥", "😂"}
	results := make([]error, len(emojis))

	start := make(chan struct{})
	var wg sync.WaitGroup
	for i, emoji := range emojis {
		wg.Add(1)
		go func(i int, emoji string) {
			defer wg.Done()
			<-start
			svc := NewAddReactionService(repo, &fakePublisher{}, transactor)
			_, results[i] = svc.Execute(ctx, ReactionInput{
				RoomID:    roomID,
				MessageID: messageID,
				UserID:    userID,
				Username:  "alice",
				Emoji:     emoji,
			})
		}(i, emoji)
	}
	close(start)
	wg.Wait()

	successes := 0
	capErrors := 0
	for _, err := range results {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrTooManyReactions):
			capErrors++
		default:
			t.Fatalf("erro inesperado: %v", err)
		}
	}
	if successes != 1 || capErrors != 1 {
		t.Fatalf("sucessos = %d, erros de limite = %d, want 1 e 1", successes, capErrors)
	}

	var distinct int
	if err := pool.QueryRow(ctx, `SELECT COUNT(DISTINCT emoji) FROM message_reactions WHERE message_id = $1`, messageID).Scan(&distinct); err != nil {
		t.Fatalf("contar emojis: %v", err)
	}
	if distinct != maxDistinctEmojis {
		t.Errorf("emojis distintos = %d, want %d", distinct, maxDistinctEmojis)
	}
}

func TestAddReactionService_LockTimeout(t *testing.T) {
	dbURL := os.Getenv("VOZZERA_TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("VOZZERA_TEST_DATABASE_URL não definido; pulando teste de timeout de lock")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("conectar no banco: %v", err)
	}
	defer pool.Close()

	roomID, messageID, userID, err := seedReactionMessage(ctx, pool, 0)
	if err != nil {
		t.Fatalf("preparar fixture: %v", err)
	}

	holder, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("reservar conexão: %v", err)
	}
	defer holder.Release()

	held, err := holder.Begin(ctx)
	if err != nil {
		t.Fatalf("abrir transação concorrente: %v", err)
	}
	defer held.Rollback(ctx)

	if _, err := held.Exec(ctx, `SELECT id FROM messages WHERE id = $1 FOR UPDATE`, messageID); err != nil {
		t.Fatalf("travar mensagem: %v", err)
	}

	svc := NewAddReactionService(New(pool), &fakePublisher{}, NewPgTransactor(pool))
	started := time.Now()
	_, err = svc.Execute(ctx, ReactionInput{
		RoomID:    roomID,
		MessageID: messageID,
		UserID:    userID,
		Username:  "alice",
		Emoji:     "🔥",
	})
	elapsed := time.Since(started)

	var httpErr *httpx.Error
	if !errors.As(err, &httpErr) {
		t.Fatalf("erro = %v, want erro HTTP nomeado", err)
	}
	if httpErr.Status != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", httpErr.Status, http.StatusServiceUnavailable)
	}
	if !strings.Contains(err.Error(), sqlStateLockNotAvailable) {
		t.Errorf("erro = %v, want SQLSTATE %s", err, sqlStateLockNotAvailable)
	}
	if elapsed > reactionLockTimeout+2*time.Second {
		t.Errorf("espera = %v, want cerca de %v", elapsed, reactionLockTimeout)
	}
}

func seedReactionMessage(ctx context.Context, pool *pgxpool.Pool, reactionCount int) (uuid.UUID, uuid.UUID, uuid.UUID, error) {
	var userID uuid.UUID
	err := pool.QueryRow(ctx,
		`INSERT INTO users (username, password_hash, email) VALUES ($1, 'x', $2) RETURNING id`,
		"user-"+uuid.NewString(), uuid.NewString()+"@test.local",
	).Scan(&userID)
	if err != nil {
		return uuid.Nil, uuid.Nil, uuid.Nil, err
	}

	var roomID uuid.UUID
	err = pool.QueryRow(ctx,
		`INSERT INTO rooms (name) VALUES ($1) RETURNING id`,
		"room-"+uuid.NewString(),
	).Scan(&roomID)
	if err != nil {
		return uuid.Nil, uuid.Nil, uuid.Nil, err
	}

	var messageID uuid.UUID
	err = pool.QueryRow(ctx,
		`INSERT INTO messages (room_id, user_id, content) VALUES ($1, $2, 'oi') RETURNING id`,
		roomID, userID,
	).Scan(&messageID)
	if err != nil {
		return uuid.Nil, uuid.Nil, uuid.Nil, err
	}

	_, err = pool.Exec(ctx,
		`INSERT INTO message_reactions (message_id, user_id, emoji)
		 SELECT $1, $2, 'seed-' || g FROM generate_series(1, $3) AS g`,
		messageID, userID, reactionCount,
	)
	if err != nil {
		return uuid.Nil, uuid.Nil, uuid.Nil, err
	}

	return roomID, messageID, userID, nil
}
