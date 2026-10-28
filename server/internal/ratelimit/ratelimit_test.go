package ratelimit

import (
	"testing"
	"time"
)

func TestBurstThenRefill(t *testing.T) {
	now := time.Unix(0, 0)
	l := New(5, time.Minute, 3) // one token every 12s, burst 3
	l.now = func() time.Time { return now }

	for i := range 3 {
		if ok, _ := l.Allow("ip"); !ok {
			t.Fatalf("request %d within burst was refused", i+1)
		}
	}
	ok, retry := l.Allow("ip")
	if ok {
		t.Fatal("4th request should exceed the burst")
	}
	if retry <= 0 || retry > 12*time.Second {
		t.Errorf("retryAfter = %v, want (0, 12s]", retry)
	}

	if ok, _ := l.Allow("other-ip"); !ok {
		t.Fatal("keys must not share buckets")
	}

	now = now.Add(12 * time.Second)
	if ok, _ := l.Allow("ip"); !ok {
		t.Fatal("a token should have refilled after 12s")
	}
}

func TestIdleBucketsAreDropped(t *testing.T) {
	now := time.Unix(0, 0)
	l := New(1, time.Second, 1)
	l.now = func() time.Time { return now }

	l.Allow("a")
	now = now.Add(idleAfter + time.Second)
	l.Allow("b")
	if _, found := l.buckets["a"]; found {
		t.Fatal("idle bucket should have been collected")
	}
}
