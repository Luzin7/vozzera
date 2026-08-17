package chat

import (
	"net/http"

	"github.com/Luzin7/vozzera-backend/internal/shared/httpx"
)

var (
	ErrNotAuthorized       = httpx.Errorf(http.StatusUnauthorized, "Não autorizado")
	ErrInvalidContent      = httpx.Errorf(http.StatusBadRequest, "Mensagem deve ter entre 1 e 4000 caracteres")
	ErrEmptyContent        = httpx.Errorf(http.StatusBadRequest, "O conteúdo não pode ser vazio")
	ErrContentTooLong      = httpx.Errorf(http.StatusBadRequest, "O conteúdo deve ter no máximo 4000 caracteres")
	ErrNameRequired        = httpx.Errorf(http.StatusBadRequest, "Nome é obrigatório")
	ErrNameTooLong         = httpx.Errorf(http.StatusBadRequest, "Nome deve ter no máximo 100 caracteres")
	ErrInvalidRoomType     = httpx.Errorf(http.StatusBadRequest, "Tipo deve ser 'text' ou 'voice'")
	ErrRoomNotFound        = httpx.Errorf(http.StatusNotFound, "Sala não encontrada")
	ErrMessageNotEditable  = httpx.Errorf(http.StatusForbidden, "Mensagem não encontrada ou você não tem permissão para editá-la")
	ErrMessageNotDeletable = httpx.Errorf(http.StatusForbidden, "Mensagem não encontrada ou você não tem permissão para deletá-la")
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

func ErrGetMessages(err error) error {
	return httpx.Wrapf(http.StatusInternalServerError, "Erro ao buscar mensagens", err)
}

func ErrUpdateMessage(err error) error {
	return httpx.Wrapf(http.StatusInternalServerError, "Erro interno ao atualizar mensagem", err)
}

func ErrDeleteMessage(err error) error {
	return httpx.Wrapf(http.StatusInternalServerError, "Erro interno ao deletar mensagem", err)
}

func ErrCreateMessage(err error) error {
	return httpx.Wrapf(http.StatusInternalServerError, "Erro ao salvar mensagem", err)
}
