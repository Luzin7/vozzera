package chat

import (
	"context"

	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
)

type fakePublisher struct {
	envelopes []realtime.Envelope
	topics    []realtime.Topic
	err       error
}

func (f *fakePublisher) Publish(_ context.Context, topic realtime.Topic, env realtime.Envelope) error {
	f.topics = append(f.topics, topic)
	f.envelopes = append(f.envelopes, env)
	return f.err
}

var _ realtime.Publisher = (*fakePublisher)(nil)
