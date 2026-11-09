package chat

import (
	"errors"
	"net/http"

	"github.com/Luzin7/vozzera-backend/internal/shared/httpx"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrInvalidContent      = httpx.Errorf(http.StatusBadRequest, "Mensagem deve ter entre 1 e 4000 caracteres")
	ErrEmptyContent        = httpx.Errorf(http.StatusBadRequest, "O conteúdo não pode ser vazio")
	ErrContentTooLong      = httpx.Errorf(http.StatusBadRequest, "O conteúdo deve ter no máximo 4000 caracteres")
	ErrInvalidCursor       = httpx.Errorf(http.StatusBadRequest, "Cursor de paginação inválido")
	ErrInvalidReplyTarget  = httpx.Errorf(http.StatusBadRequest, "Mensagem de resposta inválida")
	ErrRoomNotFound        = httpx.Errorf(http.StatusNotFound, "Sala não encontrada")
	ErrMessageNotEditable  = httpx.Errorf(http.StatusForbidden, "Mensagem não encontrada ou você não tem permissão para editá-la")
	ErrMessageNotDeletable = httpx.Errorf(http.StatusForbidden, "Mensagem não encontrada ou você não tem permissão para deletá-la")
	ErrMessageNotFound     = httpx.Errorf(http.StatusNotFound, "Mensagem não encontrada")
	ErrEmptyEmoji          = httpx.Errorf(http.StatusBadRequest, "O emoji não pode ser vazio")
	ErrEmojiTooLong        = httpx.Errorf(http.StatusBadRequest, "O emoji deve ter no máximo 10 caracteres")
	ErrTooManyReactions    = httpx.Errorf(http.StatusBadRequest, "A mensagem já tem o máximo de 20 emojis diferentes")
)

const (
	sqlStateLockNotAvailable = "55P03"
	sqlStateQueryCanceled    = "57014"

	reactionConflictMessage = "A mensagem está em uso por outra operação; tente novamente"
)

func isReactionContention(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == sqlStateLockNotAvailable || pgErr.Code == sqlStateQueryCanceled
}

func ErrReactionConflict(err error) error {
	return httpx.Wrapf(http.StatusServiceUnavailable, reactionConflictMessage, err)
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

func ErrAddReaction(err error) error {
	if isReactionContention(err) {
		return ErrReactionConflict(err)
	}
	return httpx.Wrapf(http.StatusInternalServerError, "Erro interno ao adicionar reação", err)
}

func ErrRemoveReaction(err error) error {
	return httpx.Wrapf(http.StatusInternalServerError, "Erro interno ao remover reação", err)
}

func ErrGetReplyTarget(err error) error {
	return httpx.Wrapf(http.StatusInternalServerError, "Erro ao validar mensagem de resposta", err)
}

func ErrAuthorizeRoom(err error) error {
	return httpx.Wrapf(http.StatusInternalServerError, "Erro ao verificar acesso à sala", err)
}
