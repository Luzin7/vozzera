package voice

import (
	"net/http"

	"github.com/Luzin7/vozzera-backend/internal/shared/httpx"
)

func ErrIssueToken(err error) error {
	return httpx.Wrapf(http.StatusInternalServerError, "Erro ao gerar token", err)
}
