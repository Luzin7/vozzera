package sendgrid

import (
	"context"
	"errors"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

const (
	dispatcherQueueSize    = 64
	dispatcherDrainTimeout = 5 * time.Second
)

type MailSender interface {
	Send(to, subject, html string) error
}

type mailProviderError interface {
	error
	HTTPStatus() int
	ResponseBody() string
}

type MailDispatcher struct {
	sender       MailSender
	health       *Health
	jobs         chan mailJob
	done         chan struct{}
	drainTimeout time.Duration
	pending      atomic.Int64

	mu     sync.Mutex
	closed bool
}

type mailJob struct {
	to      string
	subject string
	html    string
}

func NewMailDispatcher(sender MailSender, health *Health) *MailDispatcher {
	d := &MailDispatcher{
		sender:       sender,
		health:       health,
		jobs:         make(chan mailJob, dispatcherQueueSize),
		done:         make(chan struct{}),
		drainTimeout: dispatcherDrainTimeout,
	}
	go d.run()
	return d
}

func (d *MailDispatcher) Send(to, subject, html string) error {
	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		log.Printf("email.dispatch.rejected level=error reason=dispatcher_closed")
		d.health.RecordFailure(http.StatusServiceUnavailable)
		return nil
	}
	defer d.mu.Unlock()

	select {
	case d.jobs <- mailJob{to: to, subject: subject, html: html}:
		d.pending.Add(1)
	default:
		log.Printf("email.dispatch.rejected level=error reason=queue_full capacity=%d", dispatcherQueueSize)
		d.health.RecordFailure(http.StatusServiceUnavailable)
	}

	return nil
}

func (d *MailDispatcher) Close(ctx context.Context) {
	d.mu.Lock()
	if !d.closed {
		d.closed = true
		close(d.jobs)
	}
	d.mu.Unlock()

	drainCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), d.drainTimeout)
	defer cancel()

	select {
	case <-d.done:
	case <-drainCtx.Done():
		log.Printf("email.dispatch.drain_timeout level=error undelivered=%d timeout=%s", d.pending.Load(), d.drainTimeout)
	}
}

func (d *MailDispatcher) run() {
	defer close(d.done)

	for job := range d.jobs {
		d.deliver(job)
		d.pending.Add(-1)
	}
}

func (d *MailDispatcher) deliver(job mailJob) {
	err := d.sender.Send(job.to, job.subject, job.html)
	if err == nil {
		d.health.RecordSuccess()
		return
	}

	var providerErr mailProviderError
	if errors.As(err, &providerErr) {
		log.Printf("email.dispatch.failed level=error provider_status=%d body=%q", providerErr.HTTPStatus(), providerErr.ResponseBody())
		d.health.RecordFailure(providerErr.HTTPStatus())
		return
	}

	log.Printf("email.dispatch.failed level=error err=%q", err)
	d.health.RecordFailure(http.StatusServiceUnavailable)
}
