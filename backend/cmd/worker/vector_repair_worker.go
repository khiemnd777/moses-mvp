package main

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	appconfig "github.com/khiemnd777/legal_api/pkg/config"
)

const (
	defaultVectorRepairInterval        = 30 * time.Second
	defaultVectorRepairMaxTasksPerPass = 20
)

type vectorRepairPassFunc func(ctx context.Context, limit int) (int, error)

type vectorRepairTicker struct {
	ch   <-chan time.Time
	stop func()
}

type vectorRepairWorker struct {
	logger             *slog.Logger
	enabled            bool
	interval           time.Duration
	maxTasksPerPass    int
	runPass            vectorRepairPassFunc
	newTicker          func(interval time.Duration) vectorRepairTicker
	intervalDefaulted  bool
	configuredInterval time.Duration

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func newVectorRepairWorker(logger *slog.Logger, cfg appconfig.VectorRepairConfig, runPass vectorRepairPassFunc) *vectorRepairWorker {
	interval := cfg.Interval
	intervalDefaulted := false
	if interval <= 0 {
		interval = defaultVectorRepairInterval
		intervalDefaulted = true
	}
	maxTasksPerPass := cfg.MaxTasksPerPass
	if maxTasksPerPass <= 0 {
		maxTasksPerPass = defaultVectorRepairMaxTasksPerPass
	}
	return &vectorRepairWorker{
		logger:             logger,
		enabled:            cfg.Enabled,
		interval:           interval,
		maxTasksPerPass:    maxTasksPerPass,
		runPass:            runPass,
		intervalDefaulted:  intervalDefaulted,
		configuredInterval: cfg.Interval,
		newTicker: func(interval time.Duration) vectorRepairTicker {
			ticker := time.NewTicker(interval)
			return vectorRepairTicker{
				ch: ticker.C,
				stop: func() {
					ticker.Stop()
				},
			}
		},
	}
}

func (w *vectorRepairWorker) Start(parent context.Context) bool {
	if !w.enabled {
		w.logger.Info("vector_repair_worker_disabled")
		return false
	}
	if w.intervalDefaulted {
		w.logger.Warn(
			"vector_repair_worker_invalid_interval",
			slog.Duration("configured_interval", w.configuredInterval),
			slog.Duration("fallback_interval", w.interval),
		)
	}
	workerCtx, cancel := context.WithCancel(parent)
	w.cancel = cancel
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		w.run(workerCtx)
	}()
	return true
}

func (w *vectorRepairWorker) Stop() {
	if w.cancel != nil {
		w.cancel()
	}
	w.wg.Wait()
}

func (w *vectorRepairWorker) Wait() {
	w.wg.Wait()
}

func (w *vectorRepairWorker) run(ctx context.Context) {
	ticker := w.newTicker(w.interval)
	defer ticker.stop()
	w.logger.Info(
		"vector_repair_worker_started",
		slog.Bool("enabled", w.enabled),
		slog.Duration("interval", w.interval),
		slog.Int("max_tasks_per_pass", w.maxTasksPerPass),
	)
	defer w.logger.Info("vector_repair_worker_stopped")

	w.runPassSafely(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.ch:
			w.runPassSafely(ctx)
		}
	}
}

func (w *vectorRepairWorker) runPassSafely(ctx context.Context) {
	if _, err := w.runPass(ctx, w.maxTasksPerPass); err != nil && !errors.Is(err, context.Canceled) {
		w.logger.Error("failed to run vector repair pass", slog.String("error", err.Error()))
	}
}
