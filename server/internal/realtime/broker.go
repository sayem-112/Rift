// Package realtime fans change notifications out to open event streams.
//
// Writers call pg_notify('rift_changes', <user id>) inside their transaction
// (see library.record); Postgres delivers it to every listening connection
// when the transaction commits. Each API instance holds one LISTEN connection
// and wakes the streams of that user that it is serving. So a position saved
// through instance A reaches a phone connected to instance B, with no message
// broker.
//
// Notifications carry no data, only "this user changed". Streams then read
// the change feed from their own cursor, so a dropped or coalesced
// notification can never lose a change; it only delays it to the next one or
// to the stream's periodic check.
package realtime

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const Channel = "rift_changes"

type Broker struct {
	dsn string
	log *slog.Logger

	mu   sync.Mutex
	subs map[uuid.UUID]map[chan struct{}]struct{}

	ready     chan struct{}
	readyOnce sync.Once
}

func NewBroker(dsn string, log *slog.Logger) *Broker {
	return &Broker{dsn: dsn, log: log, subs: make(map[uuid.UUID]map[chan struct{}]struct{}), ready: make(chan struct{})}
}

// Ready is closed once the first LISTEN is established.
func (b *Broker) Ready() <-chan struct{} { return b.ready }

// Subscribe returns a channel that receives a signal whenever the user's data
// may have changed. Signals coalesce: one pending signal stands for any number
// of changes. Call cancel when the stream ends.
func (b *Broker) Subscribe(userID uuid.UUID) (signal <-chan struct{}, cancel func()) {
	ch := make(chan struct{}, 1)
	b.mu.Lock()
	if b.subs[userID] == nil {
		b.subs[userID] = make(map[chan struct{}]struct{})
	}
	b.subs[userID][ch] = struct{}{}
	b.mu.Unlock()

	return ch, func() {
		b.mu.Lock()
		delete(b.subs[userID], ch)
		if len(b.subs[userID]) == 0 {
			delete(b.subs, userID)
		}
		b.mu.Unlock()
	}
}

func (b *Broker) wake(userID uuid.UUID) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs[userID] {
		select {
		case ch <- struct{}{}:
		default: // a signal is already pending
		}
	}
}

// wakeAll nudges every stream, e.g. after reconnecting, when notifications
// sent while disconnected were lost.
func (b *Broker) wakeAll() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, set := range b.subs {
		for ch := range set {
			select {
			case ch <- struct{}{}:
			default:
			}
		}
	}
}

// Run holds the LISTEN connection until ctx ends, reconnecting with backoff.
func (b *Broker) Run(ctx context.Context) {
	backoff := time.Second
	for ctx.Err() == nil {
		err := b.listen(ctx)
		if ctx.Err() != nil {
			return
		}
		b.log.Warn("change listener disconnected; reconnecting", "err", err, "in", backoff)
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, 30*time.Second)
	}
}

func (b *Broker) listen(ctx context.Context) error {
	conn, err := pgx.Connect(ctx, b.dsn)
	if err != nil {
		return err
	}
	defer conn.Close(context.WithoutCancel(ctx))
	if _, err := conn.Exec(ctx, "LISTEN "+Channel); err != nil {
		return err
	}
	b.log.Info("listening for changes", "channel", Channel)
	b.readyOnce.Do(func() { close(b.ready) })
	// Anything committed while we were away was never delivered to us.
	b.wakeAll()

	for {
		n, err := conn.WaitForNotification(ctx)
		if err != nil {
			return err
		}
		if id, err := uuid.Parse(n.Payload); err == nil {
			b.wake(id)
		}
	}
}
