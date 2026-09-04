package observability

import (
	"context"

	"github.com/google/uuid"
)

// WithRequestID creates a new context with a generated request ID.
func WithRequestID(ctx context.Context) context.Context {
	id := uuid.New().String()[:8]
	return context.WithValue(ctx, RequestIDKey{}, id)
}

// WithRunID creates a new context with a generated run ID and command name.
func WithRunID(ctx context.Context, command string) context.Context {
	id := uuid.New().String()[:8]
	ctx = context.WithValue(ctx, RunIDKey{}, id)
	ctx = context.WithValue(ctx, CommandKey{}, command)

	return ctx
}

// RequestID extracts the request/run ID from context.
func RequestID(ctx context.Context) string {
	if id, ok := ctx.Value(RequestIDKey{}).(string); ok {
		return id
	}

	if id, ok := ctx.Value(RunIDKey{}).(string); ok {
		return id
	}

	return ""
}

// RunID extracts only the run ID from context (does not fall back to request ID).
func RunID(ctx context.Context) string {
	if id, ok := ctx.Value(RunIDKey{}).(string); ok {
		return id
	}

	return ""
}

// Command extracts the command name from context.
func Command(ctx context.Context) string {
	if cmd, ok := ctx.Value(CommandKey{}).(string); ok {
		return cmd
	}

	return ""
}

// WithCommand creates a new context with the given command name.
func WithCommand(ctx context.Context, command string) context.Context {
	return context.WithValue(ctx, CommandKey{}, command)
}
