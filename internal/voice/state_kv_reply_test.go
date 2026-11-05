package voice

import (
	"errors"
	"testing"
)

func TestActiveCountFromReply(t *testing.T) {
	count, err := activeCountFrom(int64(0))
	if err != nil || count != 0 {
		t.Fatalf("count = %d, err = %v, want 0, nil", count, err)
	}

	for name, reply := range map[string]any{"string": "0", "nil": nil, "float": 0.0} {
		if _, err := activeCountFrom(reply); !errors.Is(err, errUnexpectedScriptReply) {
			t.Errorf("%s: err = %v, want errUnexpectedScriptReply", name, err)
		}
	}
}

func TestHydrateResultFromReply(t *testing.T) {
	result, err := hydrateResultFrom([]any{int64(1), "1700000000123"})
	if err != nil || !result.Written || result.SessionStartedAt == nil {
		t.Fatalf("result = %+v, err = %v, want Written com sessão", result, err)
	}

	for name, reply := range map[string][]any{
		"flag string": {"1", ""},
		"flag nil":    {nil, ""},
		"tamanho":     {int64(1)},
	} {
		if _, err := hydrateResultFrom(reply); !errors.Is(err, errUnexpectedScriptReply) {
			t.Errorf("%s: err = %v, want errUnexpectedScriptReply", name, err)
		}
	}
}
