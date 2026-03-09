package api

import (
	"context"
	"time"

	"github.com/khiemnd777/legal_api/observability"
)

func (h *Handler) startAnswerTrace(ctx context.Context, mode, question string) (context.Context, *observability.TraceService, string, error) {
	traceID := observability.TraceIDFromContext(ctx)
	traceSvc, err := observability.NewTraceService(ctx, h.TraceRepo, h.Logger, traceID, mode, question)
	if err != nil {
		return ctx, nil, traceID, err
	}
	ctx = observability.WithRecorder(ctx, traceSvc)
	return ctx, traceSvc, traceID, nil
}

func traceLatency(started time.Time) time.Duration {
	if started.IsZero() {
		return 0
	}
	return time.Since(started)
}
