package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/Luzin7/vozzera-backend/internal/auth"
	"github.com/Luzin7/vozzera-backend/internal/chat"
	"github.com/Luzin7/vozzera-backend/internal/shared/config"
	shareddb "github.com/Luzin7/vozzera-backend/internal/shared/db"
	"github.com/Luzin7/vozzera-backend/internal/shared/httpx"
	"github.com/Luzin7/vozzera-backend/internal/swagger"
	"github.com/Luzin7/vozzera-backend/internal/voice"
)

func main() {
	cfg := config.Load()

	ctx := context.Background()
	pool, err := shareddb.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Erro ao conectar no banco: %v", err)
	}
	defer pool.Close()

	authQueries := auth.New(pool)
	chatQueries := chat.New(pool)
	voiceQueries := voice.New(pool)

	hub := chat.NewHub(chatQueries)
	go hub.Run()
	go cleanupExpiredSessions(authQueries)

	mux := http.NewServeMux()

	authMw := httpx.Auth(func(ctx context.Context, raw string) (httpx.UserClaims, error) {
		sid, err := uuid.Parse(raw)
		if err != nil {
			return httpx.UserClaims{}, errors.New("cookie de sessão inválido")
		}

		session, err := authQueries.GetSessionByID(ctx, sid)
		if err != nil {
			return httpx.UserClaims{}, err
		}

		if time.Now().After(session.ExpiresAt) {
			return httpx.UserClaims{}, errors.New("sessão expirada")
		}

		if time.Until(session.ExpiresAt) < cfg.SessionTouchWindow {
			_ = authQueries.TouchSession(ctx, auth.TouchSessionParams{
				ID:        sid,
				ExpiresAt: time.Now().Add(cfg.SessionTTL),
			})
		}

		return httpx.UserClaims{
			UserID:    session.UserID,
			Username:  session.Username,
			SessionID: sid,
		}, nil
	})

	issuer := voice.NewTokenIssuer(cfg.LiveKitAPIKey, cfg.LiveKitAPISecret)

	rateLimiter := httpx.NewRateLimiter(map[string]httpx.RateLimitRule{
		"/api/login":       {Limit: 10, Window: time.Minute},
		"/api/register":    {Limit: 5, Window: time.Minute},
		"/api/logout":      {Limit: 30, Window: time.Minute},
		"/api/voice/token": {Limit: 30, Window: time.Minute},
		"/api/rooms":       {Limit: 120, Window: time.Minute},
		"/api/rooms/":      {Limit: 120, Window: time.Minute},
		"/api/voice/rooms": {Limit: 60, Window: time.Minute},
	})

	auth.RegisterHandlers(mux, authQueries, cfg.InviteCode, cfg.SessionTTL, hub.Revoke, authMw)
	chat.RegisterHandlers(mux, chatQueries, hub, authMw)
	voice.RegisterHandlers(mux, voiceQueries, issuer, cfg.LiveKitURL, authMw)
	swagger.RegisterHandlers(mux)

	mux.Handle("GET /ws", authMw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := httpx.UserFromContext(r.Context())
		if !ok {
			http.Error(w, "Não autenticado", http.StatusUnauthorized)
			return
		}
		chat.ServeWs(hub, w, r, user.UserID, user.Username, user.SessionID)
	})))

	handler := httpx.SecurityHeaders(rateLimiter.Middleware(httpx.CORS(cfg.CORSOrigins)(mux)))

	log.Printf("Servidor rodando na porta :%s", cfg.Port)

	finalHandler := httpx.Logger(handler)
	if err := http.ListenAndServe(":"+cfg.Port, finalHandler); err != nil {
		log.Fatal(err)
	}
}

func cleanupExpiredSessions(queries *auth.Queries) {
	ticker := time.NewTicker(time.Hour * 24)
	defer ticker.Stop()

	for range ticker.C {
		if err := queries.CleanupExpiredSessions(context.Background()); err != nil {
			log.Printf("erro ao limpar sessões expiradas: %v", err)
		}
	}
}
