package ws

import (
	"context"
	"encoding/json"

	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
)

type HandlerFunc func(
	ctx context.Context,
	client *realtime.Client,
	data json.RawMessage,
) error

type Router struct {
	handlers map[string]HandlerFunc
}

func NewRouter() *Router {
	return &Router{
		handlers: make(map[string]HandlerFunc),
	}
}

func (r *Router) Handle(cmd string, handler HandlerFunc) {
	r.handlers[cmd] = handler
}

func (r *Router) Dispatch(
	client *realtime.Client,
	env realtime.Envelope,
) error {
	handler, exists := r.handlers[string(env.Type)]
	if !exists {
		return nil
	}

	return handler(
		client.Context(),
		client,
		env.Data,
	)
}

func (r *Router) HandleMessage(
	client *realtime.Client,
	env realtime.Envelope,
) error {
	return r.Dispatch(client, env)
}
