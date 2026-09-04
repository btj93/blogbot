package telegram

import (
	"context"
	"errors"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func TestParseRetryError429(t *testing.T) {
	c := &Client{maxRetries: 5}
	err := &tgbotapi.Error{Code: 429, Message: `{"retry_after": 10}`}

	delay, shouldRetry := c.parseRetryError(err)
	if !shouldRetry {
		t.Error("expected shouldRetry=true for 429")
	}

	if delay != 15*time.Second {
		t.Errorf("got delay=%v, want 15s", delay)
	}
}

func TestParseRetryError400GroupSendFailed(t *testing.T) {
	c := &Client{maxRetries: 5}
	err := &tgbotapi.Error{Code: 400, Message: "group send failed"}

	delay, shouldRetry := c.parseRetryError(err)
	if !shouldRetry {
		t.Error("expected shouldRetry=true for 400 group send failed")
	}

	if delay != 5*time.Second {
		t.Errorf("got delay=%v, want 5s", delay)
	}
}

func TestParseRetryErrorNonRetryable(t *testing.T) {
	c := &Client{maxRetries: 5}
	err := &tgbotapi.Error{Code: 403, Message: "Forbidden"}

	_, shouldRetry := c.parseRetryError(err)
	if shouldRetry {
		t.Error("expected shouldRetry=false for 403")
	}
}

func TestParseRetryError_429WithoutRetryAfter(t *testing.T) {
	c := &Client{maxRetries: 3}
	apiErr := &tgbotapi.Error{Code: 429, Message: "rate limited"}

	delay, shouldRetry := c.parseRetryError(apiErr)
	if !shouldRetry {
		t.Error("expected shouldRetry=true for 429 without retry_after")
	}

	if delay != 30*time.Second {
		t.Errorf("got delay=%v, want 30s fallback", delay)
	}
}

func TestParseRetryError_NonAPIError(t *testing.T) {
	c := &Client{maxRetries: 3}
	err := errors.New("network timeout")

	_, shouldRetry := c.parseRetryError(err)
	if shouldRetry {
		t.Error("expected shouldRetry=false for non-API error")
	}
}

func TestLogText_NoLogChatID(t *testing.T) {
	c := &Client{logChatID: 0}
	// Should take the slog branch and not panic (bot is nil but never called).
	c.LogText(context.Background(), "test message")
}
