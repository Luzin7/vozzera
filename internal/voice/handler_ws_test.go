package voice

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
	"github.com/Luzin7/vozzera-backend/internal/transport/ws"
	"github.com/google/uuid"
)

type recordingEnterer struct {
	inputs []EnterAwayInput
}

func (r *recordingEnterer) Execute(_ context.Context, in EnterAwayInput) error {
	r.inputs = append(r.inputs, in)
	return nil
}

type recordingLeaver struct {
	inputs []LeaveAwayInput
}

func (r *recordingLeaver) Execute(_ context.Context, in LeaveAwayInput) error {
	r.inputs = append(r.inputs, in)
	return nil
}

func newAwayRouter(instanceID uuid.UUID) (*ws.Router, *recordingEnterer, *recordingLeaver) {
	enterer, leaver := &recordingEnterer{}, &recordingLeaver{}
	router := ws.NewRouter()
	RegisterAwayHandlers(router, AwayHandlerDeps{Enter: enterer, Leave: leaver, InstanceID: instanceID})
	return router, enterer, leaver
}

func TestAwayHandlers_EnterUsaIdentidadeDaSessaoEIgnoraUserIDDoPayload(t *testing.T) {
	instanceID, roomID, spoofed := uuid.New(), uuid.New(), uuid.New()
	router, enterer, _ := newAwayRouter(instanceID)
	client := newTestClient()
	data := fmt.Sprintf(`{"room_id":%q,"reason":"idle","user_id":%q,"username":"mallory"}`, roomID, spoofed)

	err := router.Dispatch(client, realtime.Envelope{Type: CmdAwayEnter, Data: json.RawMessage(data)})

	if err != nil || len(enterer.inputs) != 1 {
		t.Fatalf("err = %v, chamadas = %d, want 1", err, len(enterer.inputs))
	}
	got := enterer.inputs[0]
	if got.UserID != client.UserID || got.Username != "alice" {
		t.Errorf("identidade = %s/%s, want a da sessão", got.UserID, got.Username)
	}
	if got.Owner != (AwayOwner{InstanceID: instanceID, ConnID: client.ConnID}) {
		t.Errorf("owner = %+v, want instância e conexão da sessão", got.Owner)
	}
	if got.RoomID != roomID || got.Reason != "idle" {
		t.Errorf("sala/motivo = %s/%s", got.RoomID, got.Reason)
	}
	if got.ConnCtx != client.Context() {
		t.Error("ConnCtx deveria ser o contexto da conexão")
	}
}

func TestAwayHandlers_EnterComPayloadInvalidoNaoChamaOServico(t *testing.T) {
	router, enterer, _ := newAwayRouter(uuid.New())

	err := router.Dispatch(newTestClient(), realtime.Envelope{Type: CmdAwayEnter, Data: json.RawMessage(`{"room_id":42}`)})

	if err != nil || len(enterer.inputs) != 0 {
		t.Fatalf("err = %v, chamadas = %d, want nenhuma", err, len(enterer.inputs))
	}
}

func TestAwayHandlers_LeaveUsaUsuarioDaSessao(t *testing.T) {
	router, _, leaver := newAwayRouter(uuid.New())
	client := newTestClient()

	err := router.Dispatch(client, realtime.Envelope{Type: CmdAwayLeave, Data: json.RawMessage(fmt.Sprintf(`{"user_id":%q}`, uuid.New()))})

	if err != nil || len(leaver.inputs) != 1 || leaver.inputs[0].UserID != client.UserID {
		t.Fatalf("err = %v, inputs = %+v, want o usuário da sessão", err, leaver.inputs)
	}
}
