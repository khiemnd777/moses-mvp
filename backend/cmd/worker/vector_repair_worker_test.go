package main

import (
	"context"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	appconfig "github.com/khiemnd777/legal_api/pkg/config"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func waitForCondition(t *testing.T, timeout time.Duration, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("condition not met within %s", timeout)
}

func TestVectorRepairWorkerDoesNotStartWhenDisabled(t *testing.T) {
	var calls atomic.Int32
	w := newVectorRepairWorker(testLogger(), appconfig.VectorRepairConfig{
		Enabled:  false,
		Interval: 10 * time.Millisecond,
	}, func(ctx context.Context, limit int) (int, error) {
		calls.Add(1)
		return 0, nil
	})

	started := w.Start(context.Background())
	w.Stop()

	if started {
		t.Fatalf("expected worker not to start when disabled")
	}
	if got := calls.Load(); got != 0 {
		t.Fatalf("expected no pass calls, got %d", got)
	}
}

func TestVectorRepairWorkerStartsWhenEnabled(t *testing.T) {
	var calls atomic.Int32
	tickCh := make(chan time.Time, 1)
	w := newVectorRepairWorker(testLogger(), appconfig.VectorRepairConfig{
		Enabled:  true,
		Interval: 10 * time.Millisecond,
	}, func(ctx context.Context, limit int) (int, error) {
		calls.Add(1)
		return 0, nil
	})
	w.newTicker = func(interval time.Duration) vectorRepairTicker {
		return vectorRepairTicker{
			ch: tickCh,
			stop: func() {
			},
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := w.Start(ctx)
	waitForCondition(t, 500*time.Millisecond, func() bool {
		return calls.Load() >= 1
	})
	w.Stop()

	if !started {
		t.Fatalf("expected worker to start")
	}
}

func TestVectorRepairWorkerRespectsContextCancellation(t *testing.T) {
	started := make(chan struct{}, 1)
	w := newVectorRepairWorker(testLogger(), appconfig.VectorRepairConfig{
		Enabled:  true,
		Interval: time.Hour,
	}, func(ctx context.Context, limit int) (int, error) {
		select {
		case started <- struct{}{}:
		default:
		}
		<-ctx.Done()
		return 0, ctx.Err()
	})
	w.newTicker = func(interval time.Duration) vectorRepairTicker {
		return vectorRepairTicker{
			ch: make(chan time.Time),
			stop: func() {
			},
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	w.Start(ctx)
	<-started
	cancel()

	done := make(chan struct{})
	go func() {
		w.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatalf("worker did not stop after context cancellation")
	}
}

func TestVectorRepairWorkerStopsTickerOnShutdown(t *testing.T) {
	tickCh := make(chan time.Time, 1)
	stopped := make(chan struct{})
	w := newVectorRepairWorker(testLogger(), appconfig.VectorRepairConfig{
		Enabled:  true,
		Interval: time.Hour,
	}, func(ctx context.Context, limit int) (int, error) {
		return 0, nil
	})
	w.newTicker = func(interval time.Duration) vectorRepairTicker {
		return vectorRepairTicker{
			ch: tickCh,
			stop: func() {
				close(stopped)
			},
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	w.Start(ctx)
	cancel()
	w.Wait()

	select {
	case <-stopped:
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("ticker stop was not called")
	}
}

func TestVectorRepairWorkerInvalidIntervalFallsBackToDefault(t *testing.T) {
	var gotInterval time.Duration
	w := newVectorRepairWorker(testLogger(), appconfig.VectorRepairConfig{
		Enabled:  true,
		Interval: -1 * time.Second,
	}, func(ctx context.Context, limit int) (int, error) {
		return 0, nil
	})
	w.newTicker = func(interval time.Duration) vectorRepairTicker {
		gotInterval = interval
		return vectorRepairTicker{
			ch: make(chan time.Time),
			stop: func() {
			},
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	w.Start(ctx)
	cancel()
	w.Wait()

	if gotInterval != defaultVectorRepairInterval {
		t.Fatalf("expected default interval %s, got %s", defaultVectorRepairInterval, gotInterval)
	}
}

func TestVectorRepairWorkerNoOverlappingPasses(t *testing.T) {
	tickCh := make(chan time.Time, 8)
	release := make(chan struct{})
	var concurrent atomic.Int32
	var maxConcurrent atomic.Int32

	w := newVectorRepairWorker(testLogger(), appconfig.VectorRepairConfig{
		Enabled:  true,
		Interval: 10 * time.Millisecond,
	}, func(ctx context.Context, limit int) (int, error) {
		current := concurrent.Add(1)
		for {
			existing := maxConcurrent.Load()
			if current <= existing || maxConcurrent.CompareAndSwap(existing, current) {
				break
			}
		}
		select {
		case <-release:
		case <-ctx.Done():
		}
		concurrent.Add(-1)
		return 0, nil
	})
	w.newTicker = func(interval time.Duration) vectorRepairTicker {
		return vectorRepairTicker{
			ch: tickCh,
			stop: func() {
			},
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	w.Start(ctx)
	waitForCondition(t, 500*time.Millisecond, func() bool {
		return concurrent.Load() == 1
	})

	tickCh <- time.Now()
	tickCh <- time.Now()
	tickCh <- time.Now()
	close(release)
	waitForCondition(t, 500*time.Millisecond, func() bool {
		return concurrent.Load() == 0
	})

	cancel()
	w.Wait()

	if maxConcurrent.Load() > 1 {
		t.Fatalf("expected no overlapping passes, max concurrency=%d", maxConcurrent.Load())
	}
}
