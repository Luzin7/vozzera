package chat

import (
	"net/http"

	"github.com/Luzin7/vozzera-backend/internal/shared/httpx"
)

var (
	ErrInvalidContent      = httpx.Errorf(http.StatusBadRequest, "Mensagem deve ter entre 1 e 4000 caracteres")
	ErrEmptyContent        = httpx.Errorf(http.StatusBadRequest, "O conteúdo não pode ser vazio")
	ErrContentTooLong      = httpx.Errorf(http.StatusBadRequest, "O conteúdo deve ter no máximo 4000 caracteres")
	ErrRoomNotFound        = httpx.Errorf(http.StatusNotFound, "Sala não encontrada")
	ErrMessageNotEditable  = httpx.Errorf(http.StatusForbidden, "Mensagem não encontrada ou você não tem permissão para editá-la")
	ErrMessageNotDeletable = httpx.Errorf(http.StatusForbidden, "Mensagem não encontrada ou você não tem permissão para deletá-la")
)

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
