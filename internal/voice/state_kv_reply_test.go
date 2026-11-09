package voice

import (
	"errors"
	"testing"

	"github.com/google/uuid"
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
	roomID := uuid.New()
	result, err := hydrateResultFrom([]any{int64(1), "1700000000123", int64(0), []any{}}, roomID)
	if err != nil || !result.Written || result.Conflict || result.SessionStartedAt == nil {
		t.Fatalf("result = %+v, err = %v, want Written com sessão", result, err)
	}

	conflict, err := hydrateResultFrom([]any{int64(0), "", int64(1), []any{}}, roomID)
	if err != nil || !conflict.Conflict || conflict.Written {
		t.Fatalf("conflict = %+v, err = %v, want conflito", conflict, err)
	}

	userID, connID, instanceID := uuid.New(), uuid.New(), uuid.New()
	removed, err := hydrateResultFrom([]any{
		int64(1), "", int64(0),
		[]any{userID.String(), `{"username":"ana","since":1,"conn_id":"` + connID.String() + `","instance_id":"` + instanceID.String() + `"}`},
	}, roomID)
	if err != nil || len(removed.RemovedAway) != 1 || removed.RemovedAway[0].UserID != userID {
		t.Fatalf("removed = %+v, err = %v, want o ausente removido", removed, err)
	}

	for name, reply := range map[string][]any{
		"flag string":     {"1", "", int64(0), []any{}},
		"flag nil":        {nil, "", int64(0), []any{}},
		"conflict nil":    {int64(0), "", nil, []any{}},
		"conflict string": {int64(0), "", "1", []any{}},
		"away inválido":   {int64(1), "", int64(0), "não é lista"},
		"tamanho":         {int64(1), "", int64(0)},
	} {
		if _, err := hydrateResultFrom(reply, roomID); !errors.Is(err, errUnexpectedScriptReply) {
			t.Errorf("%s: err = %v, want errUnexpectedScriptReply", name, err)
		}
	}
}
