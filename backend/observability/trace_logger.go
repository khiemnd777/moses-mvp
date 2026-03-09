package observability

import (
	"context"
	"log/slog"
)

func LogInfo(ctx context.Context, logger *slog.Logger, component, message string, metadata map[string]interface{}) {
	logWithLevel(ctx, logger, slog.LevelInfo, component, message, metadata)
}

func LogError(ctx context.Context, logger *slog.Logger, component, message string, metadata map[string]interface{}) {
	logWithLevel(ctx, logger, slog.LevelError, component, message, metadata)
}

func LogWarn(ctx context.Context, logger *slog.Logger, component, message string, metadata map[string]interface{}) {
	logWithLevel(ctx, logger, slog.LevelWarn, component, message, metadata)
}

func logWithLevel(ctx context.Context, logger *slog.Logger, level slog.Level, component, message string, metadata map[string]interface{}) {
	if logger == nil {
		logger = slog.Default()
	}
	if metadata == nil {
		metadata = map[string]interface{}{}
	}
	attrs := []any{
		"trace_id", TraceIDFromContext(ctx),
		"component", component,
		"metadata", metadata,
	}
	logger.Log(ctx, level, message, attrs...)
}
