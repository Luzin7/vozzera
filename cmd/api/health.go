package main

import (
	"encoding/json"
	"net/http"

	"github.com/Luzin7/vozzera-backend/internal/infra/mail"
)

const mailStatusDisabled = "disabled"

func registerHealthHandlers(mux *http.ServeMux, mailHealth *mail.Health) {
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		writeHealth(w, http.StatusOK, "ok", mailStatus(mailHealth))
	})
	mux.HandleFunc("GET /api/health/deep", func(w http.ResponseWriter, r *http.Request) {
		status := mailStatus(mailHealth)
		writeHealth(w, deepHealthStatus(status), deepStatus(status), status)
	})
}

func mailStatus(mailHealth *mail.Health) string {
	if mailHealth == nil {
		return mailStatusDisabled
	}
	return mailHealth.Status()
}

func deepHealthStatus(mailState string) int {
	if mailState == mail.MailStatusFailing || mailState == mailStatusDisabled {
		return http.StatusServiceUnavailable
	}
	return http.StatusOK
}

func deepStatus(mailState string) string {
	if deepHealthStatus(mailState) == http.StatusServiceUnavailable {
		return "degraded"
	}
	return "ok"
}

func writeHealth(w http.ResponseWriter, code int, status, mailState string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": status, "mail": mailState})
}
