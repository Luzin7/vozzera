package httpx

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteError(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
	}{
		{"erro mapeado", Errorf(http.StatusBadRequest, "mensagem"), http.StatusBadRequest},
		{"erro enrolado", Wrapf(http.StatusConflict, "mensagem", errors.New("raiz")), http.StatusConflict},
		{"erro desconhecido", errors.New("raiz"), http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			WriteError(w, tt.err)
			if w.Code != tt.status {
				t.Errorf("status = %d, want %d", w.Code, tt.status)
			}
		})
	}
}

func TestErrorf_Unwrap(t *testing.T) {
	base := errors.New("raiz")
	wrapped := Wrapf(http.StatusInternalServerError, "mensagem", base)
	if !errors.Is(wrapped, base) {
		t.Error("errors.Is falhou para erro enrolado")
	}
}
