package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/Luzin7/vozzera-backend/internal/auth"
	"github.com/Luzin7/vozzera-backend/internal/chat"
	"github.com/Luzin7/vozzera-backend/internal/shared/config"
	shareddb "github.com/Luzin7/vozzera-backend/internal/shared/db"
	"github.com/Luzin7/vozzera-backend/internal/shared/httpx"
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

	mux := http.NewServeMux()

	authMw := httpx.Auth(func(token string) (httpx.UserClaims, error) {
		claims, err := auth.ParseToken(cfg.JWTSecret, token)
		if err != nil {
			return httpx.UserClaims{}, err
		}
		return httpx.UserClaims{UserID: claims.UserID, Username: claims.Username}, nil
	})

	issuer := voice.NewTokenIssuer(cfg.LiveKitAPIKey, cfg.LiveKitAPISecret)

	rateLimiter := httpx.NewRateLimiter(map[string]httpx.RateLimitRule{
		"/api/login":       {Limit: 10, Window: time.Minute},
		"/api/register":    {Limit: 5, Window: time.Minute},
		"/api/voice/token": {Limit: 30, Window: time.Minute},
		"/api/rooms":       {Limit: 120, Window: time.Minute},
		"/api/rooms/":      {Limit: 120, Window: time.Minute},
		"/api/voice/rooms": {Limit: 60, Window: time.Minute},
	})

	auth.RegisterHandlers(mux, authQueries, cfg.JWTSecret, cfg.InviteCode)
	chat.RegisterHandlers(mux, chatQueries, hub, authMw)
	voice.RegisterHandlers(mux, voiceQueries, issuer, cfg.LiveKitURL, authMw)

	mux.Handle("GET /ws", authMw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := httpx.UserFromContext(r.Context())
		if !ok {
			http.Error(w, "Não autenticado", http.StatusUnauthorized)
			return
		}
		chat.ServeWs(hub, w, r, user.UserID, user.Username)
	})))

	handler := httpx.SecurityHeaders(rateLimiter.Middleware(httpx.CORS(cfg.CORSOrigins)(mux)))

	log.Printf("Servidor rodando na porta :%s", cfg.Port)

	finalHandler := httpx.Logger(handler)
	if err := http.ListenAndServe(":"+cfg.Port, finalHandler); err != nil {
		log.Fatal(err)
	}
}
