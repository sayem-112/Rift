package realtime

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestSignalsCoalesceAndTargetOneUser(t *testing.T) {
	b := NewBroker("", slog.New(slog.NewTextHandler(io.Discard, nil)))
	alice, bob := uuid.New(), uuid.New()
	a, cancelA := b.Subscribe(alice)
	defer cancelA()
	bb, cancelB := b.Subscribe(bob)
	defer cancelB()

	for range 5 {
		b.wake(alice)
	}
	select {
	case <-a:
	default:
		t.Fatal("alice's stream should be signalled")
	}
	select {
	case <-a:
		t.Fatal("five wakes should coalesce into one pending signal")
	default:
	}
	select {
	case <-bb:
		t.Fatal("bob's stream must not be woken by alice's changes")
	default:
	}
}

func TestCancelRemovesSubscription(t *testing.T) {
	b := NewBroker("", slog.New(slog.NewTextHandler(io.Discard, nil)))
	user := uuid.New()
	_, cancel := b.Subscribe(user)
	cancel()
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.subs) != 0 {
		t.Fatal("cancelled subscriptions must be released")
	}
}

func TestRunStopsWithContext(t *testing.T) {
	b := NewBroker("postgres://invalid:1/none", slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	done := make(chan struct{})
	go func() { b.Run(ctx); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Run must return when its context ends, even while reconnecting")
	}
}
