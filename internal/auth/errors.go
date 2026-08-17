package auth

import (
	"net/http"

	"github.com/Luzin7/vozzera-backend/internal/shared/httpx"
)

func ErrInvalidInviteCode() error {
	return httpx.Errorf(http.StatusForbidden, "Código de convite inválido")
}

func ErrInvalidUsername() error {
	return httpx.Errorf(http.StatusBadRequest, "Username deve ter entre 3 e 50 caracteres")
}

func ErrUsernameTooLong() error {
	return httpx.Errorf(http.StatusBadRequest, "Username inválido")
}

func ErrInvalidPassword() error {
	return httpx.Errorf(http.StatusBadRequest, "Senha deve ter entre 8 e 72 caracteres")
}

func ErrPasswordTooLong() error {
	return httpx.Errorf(http.StatusBadRequest, "Senha inválida")
}

func ErrHashPassword(err error) error {
	return httpx.Wrapf(http.StatusInternalServerError, "Erro ao processar senha", err)
}

func ErrUsernameTaken(err error) error {
	return httpx.Wrapf(http.StatusConflict, "Erro ao criar usuário ou username/email já em uso", err)
}

func ErrInvalidCredentials() error {
	return httpx.Errorf(http.StatusUnauthorized, "Credenciais inválidas")
}

func ErrCreateSession(err error) error {
	return httpx.Wrapf(http.StatusInternalServerError, "Erro ao criar sessão", err)
}

func ErrGetUser(err error) error {
	return httpx.Wrapf(http.StatusInternalServerError, "Erro ao buscar usuário", err)
}

func ErrUserNotFound() error {
	return httpx.Errorf(http.StatusUnauthorized, "Usuário não encontrado")
}

func ErrRevokeSession(err error) error {
	return httpx.Wrapf(http.StatusInternalServerError, "Erro ao revogar sessão", err)
}

func ErrInvalidEmail() error {
	return httpx.Errorf(http.StatusBadRequest, "Email inválido")
}

func ErrInvalidResetToken() error {
	return httpx.Errorf(http.StatusBadRequest, "Token inválido ou expirado")
}

func ErrCreateResetToken(err error) error {
	return httpx.Wrapf(http.StatusInternalServerError, "Erro ao gerar token de recuperação", err)
}

func ErrResetPassword(err error) error {
	return httpx.Wrapf(http.StatusInternalServerError, "Erro ao redefinir senha", err)
}
