package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/Luzin7/vozzera-backend/internal/auth"
	"github.com/Luzin7/vozzera-backend/internal/chat"
	"github.com/Luzin7/vozzera-backend/internal/infra/sendgrid"
	"github.com/Luzin7/vozzera-backend/internal/presence"
	"github.com/Luzin7/vozzera-backend/internal/shared/config"
	shareddb "github.com/Luzin7/vozzera-backend/internal/shared/db"
	"github.com/Luzin7/vozzera-backend/internal/shared/httpx"
	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
	"github.com/Luzin7/vozzera-backend/internal/swagger"
	"github.com/Luzin7/vozzera-backend/internal/voice"
)

func main() {
	cfg := config.Load()

	rootCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

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

	initCtx, initCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer initCancel()

	pool, err := shareddb.Connect(initCtx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Erro ao conectar no banco: %v", err)
	}
	defer func() {
		log.Println("Encerrando pool de conexões com o banco...")
		pool.Close()
	}()

	authQueries := auth.New(pool)
	chatQueries := chat.New(pool)
	voiceQueries := voice.New(pool)

	hub := realtime.NewHub()
	go hub.Run()

	presenceSvc := presence.NewService(hub)
	hub.SetPresence(presenceSvc)

	voicePresence := voice.NewVoiceRoomPresence()
	sender := chat.NewSendMessageService(chatQueries, hub)
	chatRouter := chat.NewChatRouter(sender, hub, chat.NewRoomAuthorizer(chatQueries), voicePresence)

	var bgWg sync.WaitGroup
	bgWg.Add(2)
	go func() {
		defer bgWg.Done()
		cleanupExpiredSessions(rootCtx, authQueries)
	}()
	go func() {
		defer bgWg.Done()
		cleanupExpiredPasswordResetTokens(rootCtx, authQueries)
	}()

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
		"/api/voice/webhook":   {Limit: 120, Window: time.Minute},
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
		Repo:           chatQueries,
		Publisher:      hub,
		Registerer:     hub,
		Handler:        chatRouter,
		AuthMW:         authMw,
		AllowedOrigins: cfg.CORSOrigins,
	})

	voice.RegisterHandlers(mux, voice.VoiceDeps{
		Repo:       voiceQueries,
		Issuer:     issuer,
		LiveKitURL: cfg.LiveKitURL,
		AuthMW:     authMw,
		ApiKey:     cfg.LiveKitAPIKey,
		ApiSecret:  cfg.LiveKitAPISecret,
		Presence:   voicePresence,
		Publisher:  hub,
	})
	swagger.RegisterHandlers(mux)

	handler := httpx.SecurityHeaders(rateLimiter.Middleware(httpx.CORS(cfg.CORSOrigins)(mux)))
	finalHandler := httpx.Logger(handler)

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           finalHandler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Printf("Servidor rodando na porta :%s", cfg.Port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case <-rootCtx.Done():
		log.Println("Sinal de encerramento recebido. Iniciando graceful shutdown...")
	case err := <-serverErr:
		log.Fatalf("Erro fatal ao iniciar servidor HTTP: %v", err)
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Erro durante HTTP Server Shutdown: %v", err)
	}

	if err := hub.Shutdown(shutdownCtx); err != nil {
		log.Printf("Erro durante drenagem do Hub de WebSockets: %v", err)
	}

	bgWg.Wait()
	log.Println("Processo finalizado com sucesso.")
}

func cleanupExpiredSessions(ctx context.Context, queries *auth.Queries) {
	ticker := time.NewTicker(time.Hour * 24)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := queries.CleanupExpiredSessions(context.Background()); err != nil {
				log.Printf("erro ao limpar sessões expiradas: %v", err)
			}
		}
	}
}

func cleanupExpiredPasswordResetTokens(ctx context.Context, queries *auth.Queries) {
	ticker := time.NewTicker(time.Hour * 24)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := queries.CleanupExpiredPasswordResetTokens(context.Background()); err != nil {
				log.Printf("erro ao limpar tokens de recuperação expirados: %v", err)
			}
		}
	}
}
