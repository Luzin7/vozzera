package room

import (
	"net/http"

	"github.com/Luzin7/vozzera-backend/internal/shared/httpx"
)

var (
	ErrNotAuthorized = httpx.Errorf(http.StatusUnauthorized, "Não autorizado")
	ErrNameRequired  = httpx.Errorf(http.StatusBadRequest, "Nome é obrigatório")
	ErrNameTooLong   = httpx.Errorf(http.StatusBadRequest, "Nome deve ter no máximo 100 caracteres")
	ErrInvalidFilter = httpx.Errorf(http.StatusBadRequest, "Filtro inválido")
	ErrRoomNotFound  = httpx.Errorf(http.StatusNotFound, "Sala não encontrada")
	ErrNotVoiceRoom  = httpx.Errorf(http.StatusBadRequest, "Esta sala não é de voz")
)

func ErrListRooms(err error) error {
	return httpx.Wrapf(http.StatusInternalServerError, "Erro ao listar salas", err)
}

func ErrCreateRoom(err error) error {
	return httpx.Wrapf(http.StatusInternalServerError, "Erro ao criar sala", err)
}

func ErrUpdateRoom(err error) error {
	return httpx.Wrapf(http.StatusInternalServerError, "Erro ao atualizar sala", err)
}

func ErrDeleteRoom(err error) error {
	return httpx.Wrapf(http.StatusInternalServerError, "Erro ao deletar sala", err)
}

func ErrGetRoom(err error) error {
	return httpx.Wrapf(http.StatusInternalServerError, "Erro interno", err)
}
