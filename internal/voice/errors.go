package voice

import (
	"net/http"

	"github.com/Luzin7/vozzera-backend/internal/shared/httpx"
)

var (
	ErrRoomNotFound = httpx.Errorf(http.StatusNotFound, "Sala não encontrada")
	ErrNotVoiceRoom = httpx.Errorf(http.StatusBadRequest, "Esta sala não é de voz")
)

func ErrGetRoom(err error) error {
	return httpx.Wrapf(http.StatusInternalServerError, "Erro interno", err)
}

func ErrIssueToken(err error) error {
	return httpx.Wrapf(http.StatusInternalServerError, "Erro ao gerar token", err)
}

func ErrListVoiceRooms(err error) error {
	return httpx.Wrapf(http.StatusInternalServerError, "Erro ao listar salas", err)
}
