package email

import (
	"strings"
	"testing"
)

func TestPasswordReset(t *testing.T) {
	subject, body := PasswordReset("https://vozzera.app", "tok&123")

	if subject != "Redefinição de senha — Vozzera" {
		t.Errorf("subject = %q", subject)
	}
	if !strings.Contains(body, "https://vozzera.app/reset?token=tok%26123") {
		t.Errorf("link de redefinição incorreto: %q", body)
	}
	if strings.Contains(body, "tok&123") {
		t.Error("token não escapado no href")
	}
	if !strings.Contains(body, "Redefinir senha") {
		t.Error("link de redefinição ausente")
	}
}

func TestWelcome(t *testing.T) {
	subject, body := Welcome("https://vozzera.app", "<script>alert(1)</script>")

	if subject != "Bem-vindo à Vozzera" {
		t.Errorf("subject = %q", subject)
	}
	if !strings.Contains(body, "&lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Error("username não escapado")
	}
	if strings.Contains(body, "<script>alert(1)</script>") {
		t.Error("script cru presente no corpo")
	}
	if !strings.Contains(body, "Acessar a plataforma") {
		t.Error("link de acesso ausente")
	}
}
