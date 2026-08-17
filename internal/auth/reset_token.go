package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
)

func generateResetToken() string {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		panic("erro ao gerar token de recuperação")
	}
	return hex.EncodeToString(buf)
}

func hashResetToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
