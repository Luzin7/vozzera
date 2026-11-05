package kvtest

import (
	"os"
	"testing"
)

const urlEnv = "KV_TEST_URL"

func URL(t *testing.T) string {
	t.Helper()

	url := os.Getenv(urlEnv)
	if url != "" {
		return url
	}
	if os.Getenv("CI") == "true" {
		t.Fatalf("%s é obrigatória no CI", urlEnv)
	}
	t.Skipf("%s não definida", urlEnv)
	return ""
}
