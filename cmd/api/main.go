package main

import (
	"context"
	"log"
	"net/http"

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

	log.Printf("Servidor rodando na porta :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, httpx.CORS(mux)); err != nil {
		log.Fatal(err)
	}
}
