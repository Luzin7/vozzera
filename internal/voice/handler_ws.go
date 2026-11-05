package voice

import (
	"context"
	"encoding/json"

	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
	"github.com/Luzin7/vozzera-backend/internal/transport/ws"
	"github.com/google/uuid"
)

type awayEnterer interface {
	Execute(ctx context.Context, in EnterAwayInput) error
}

type awayLeaver interface {
	Execute(ctx context.Context, in LeaveAwayInput) error
}

type AwayHandlerDeps struct {
	Enter      awayEnterer
	Leave      awayLeaver
	InstanceID uuid.UUID
}

func RegisterAwayHandlers(r *ws.Router, deps AwayHandlerDeps) {
	r.Handle(CmdAwayEnter, func(ctx context.Context, c *realtime.Client, data json.RawMessage) error {
		var cmd struct {
			RoomID uuid.UUID `json:"room_id"`
			Reason string    `json:"reason"`
		}
		if err := json.Unmarshal(data, &cmd); err != nil {
			return nil
		}

		return deps.Enter.Execute(ctx, EnterAwayInput{
			UserID:   c.UserID,
			Username: c.Username,
			RoomID:   cmd.RoomID,
			Reason:   cmd.Reason,
			Owner:    AwayOwner{InstanceID: deps.InstanceID, ConnID: c.ConnID},
			ConnCtx:  c.Context(),
		})
	})

	r.Handle(CmdAwayLeave, func(ctx context.Context, c *realtime.Client, _ json.RawMessage) error {
		return deps.Leave.Execute(ctx, LeaveAwayInput{UserID: c.UserID})
	})
}
