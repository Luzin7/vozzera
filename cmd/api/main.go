package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/Luzin7/vozzera-backend/internal/auth"
	"github.com/Luzin7/vozzera-backend/internal/chat"
	"github.com/Luzin7/vozzera-backend/internal/infra/sendgrid"
	"github.com/Luzin7/vozzera-backend/internal/shared/config"
	shareddb "github.com/Luzin7/vozzera-backend/internal/shared/db"
	"github.com/Luzin7/vozzera-backend/internal/shared/httpx"
	"github.com/Luzin7/vozzera-backend/internal/swagger"
	"github.com/Luzin7/vozzera-backend/internal/voice"
)

func main() {
	cfg := config.Load()

	var mailer auth.MailSender
	m, err := sendgrid.NewSendGridMailer(sendgrid.Config{
		APIKey:      cfg.SendGridConfig.APIKey,
		FromAddress: cfg.SendGridConfig.FromAddress,
		FromName:    cfg.SendGridConfig.FromName,
	})
	if err != nil {
		log.Printf("Envio de email desabilitado: %v", err)
		mailer = sendgrid.NewNoopMailer()
	} else {
		mailer = m
	}

	ctx := context.Background()
	pool, err := shareddb.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Erro ao conectar no banco: %v", err)
	}
	defer pool.Close()

	authQueries := auth.New(pool)
	chatQueries := chat.New(pool)
	voiceQueries := voice.New(pool)

	hub := chat.NewHub()
	go hub.Run()
	sender := chat.NewSendMessageService(chatQueries, hub)
	go cleanupExpiredSessions(authQueries)
	go cleanupExpiredPasswordResetTokens(authQueries)

	mux := http.NewServeMux()
	sessionAuth := auth.NewSessionAuthenticator(authQueries, cfg.SessionTouchWindow, cfg.SessionTTL)

	authMw := httpx.Auth(func(ctx context.Context, raw string) (httpx.UserClaims, error) {
		session, err := sessionAuth.AuthenticateSession(ctx, raw)
		if err != nil {
			return httpx.UserClaims{}, err
		}

		return httpx.UserClaims{
			UserID:    session.UserID,
			Username:  session.Username,
			Role:      session.Role,
			SessionID: session.ID,
		}, nil
	})

	issuer := voice.NewTokenIssuer(cfg.LiveKitAPIKey, cfg.LiveKitAPISecret)

	rateLimiter := httpx.NewRateLimiter(map[string]httpx.RateLimitRule{
		"/api/login":           {Limit: 10, Window: time.Minute},
		"/api/register":        {Limit: 5, Window: time.Minute},
		"/api/logout":          {Limit: 30, Window: time.Minute},
		"/api/forgot-password": {Limit: 5, Window: time.Minute},
		"/api/reset-password":  {Limit: 10, Window: time.Minute},
		"/api/voice/token":     {Limit: 30, Window: time.Minute},
		"/api/rooms":           {Limit: 120, Window: time.Minute},
		"/api/rooms/":          {Limit: 120, Window: time.Minute},
		"/api/voice/rooms":     {Limit: 60, Window: time.Minute},
	})

	auth.RegisterHandlers(mux, auth.AuthDeps{
		Repo:             authQueries,
		InviteCode:       cfg.InviteCode,
		SessionTTL:       cfg.SessionTTL,
		PasswordResetTTL: cfg.PasswordResetTTL,
		AppURL:           cfg.AppURL,
		Mailer:           mailer,
		Revoker:          hub,
		AuthMW:           authMw,
	})
	chat.RegisterHandlers(mux, chat.ChatDeps{
		Repo:   chatQueries,
		Hub:    hub,
		AuthMW: authMw,
	})
	voice.RegisterHandlers(mux, voice.VoiceDeps{
		Repo:       voiceQueries,
		Issuer:     issuer,
		LiveKitURL: cfg.LiveKitURL,
		AuthMW:     authMw,
	})
	swagger.RegisterHandlers(mux)

	mux.Handle("GET /ws", authMw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := httpx.UserFromContext(r.Context())
		if !ok {
			http.Error(w, "Não autenticado", http.StatusUnauthorized)
			return
		}
		chat.ServeWs(hub, sender, w, r, user.UserID, user.Username, user.SessionID)
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

func cleanupExpiredPasswordResetTokens(queries *auth.Queries) {
	ticker := time.NewTicker(time.Hour * 24)
	defer ticker.Stop()

	for range ticker.C {
		if err := queries.CleanupExpiredPasswordResetTokens(context.Background()); err != nil {
			log.Printf("erro ao limpar tokens de recuperação expirados: %v", err)
		}
	}
}
