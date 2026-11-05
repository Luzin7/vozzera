package voice

import (
	"net/http"

	"github.com/Luzin7/vozzera-backend/internal/shared/httpx"
)

var (
	ErrAwayRoomDenied    = httpx.Errorf(http.StatusForbidden, "Você não tem acesso a esta sala")
	ErrInvalidAwayReason = httpx.Errorf(http.StatusBadRequest, "Motivo de ausência inválido")
	ErrInvalidAwayRoom   = httpx.Errorf(http.StatusBadRequest, "Sala inválida")
)

func ErrIssueToken(err error) error {
	return httpx.Wrapf(http.StatusInternalServerError, "Erro ao gerar token", err)
}

func ErrVoiceStateUnavailable(err error) error {
	return httpx.Wrapf(http.StatusServiceUnavailable, "Estado de voz indisponível", err)
}
