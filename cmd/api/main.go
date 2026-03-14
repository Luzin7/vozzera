package main

import (
	"context"
	"log"
	"net/http"

	"github.com/Luzin7/vozzera-backend/internal/auth"
	"github.com/Luzin7/vozzera-backend/internal/chat"
	"github.com/Luzin7/vozzera-backend/internal/shared/config"
	shareddb "github.com/Luzin7/vozzera-backend/internal/shared/db"
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

	hub := chat.NewHub()
	go hub.Run()

	mux := http.NewServeMux()

	auth.RegisterHandlers(mux, authQueries, cfg.JWTSecret, cfg.InviteCode)

	mux.HandleFunc("GET /ws", func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("auth_token")
		if err != nil {
			http.Error(w, "Não autenticado", http.StatusUnauthorized)
			return
		}

		claims, err := auth.ParseToken(cfg.JWTSecret, cookie.Value)
		if err != nil {
			http.Error(w, "Token inválido", http.StatusUnauthorized)
			return
		}

		chat.ServeWs(hub, w, r, claims.UserID, claims.Username)
	})

	log.Printf("Servidor rodando na porta :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, mux); err != nil {
		log.Fatal(err)
	}
}
