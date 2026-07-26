package gapi

import (
	"context"
	"errors"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"strconv"
	"syscall"
	"time"

	"google.golang.org/api/googleapi"
)

const (
	maxAttempts   = 4
	baseDelay     = 500 * time.Millisecond
	maxDelay      = 16 * time.Second
	maxRetryAfter = 2 * time.Minute
)

// Reported by transfers that verify their payload, so the retry loop re-fetches instead of failing the item.
var ErrChecksumMismatch = errors.New("checksum mismatch after transfer")

// On exhaustion returns the last error; ctx cancellation during a backoff returns ctx.Err().
func Retry[T any](ctx context.Context, fn func() (T, error)) (T, error) {
	var zero T
	var err error
	for attempt := range maxAttempts {
		var v T
		v, err = fn()
		if err == nil {
			return v, nil
		}
		if !retryable(err) || attempt == maxAttempts-1 {
			break
		}
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		case <-time.After(delayFor(err, attempt)):
		}
	}
	return zero, err
}

func RetryErr(ctx context.Context, fn func() error) error {
	_, err := Retry(ctx, func() (struct{}, error) {
		return struct{}{}, fn()
	})
	return err
}

// Mid-stream body failures (resets, broken pipes, truncated reads, corrupt payloads) are transient too, and callers restart at offset 0.
func retryable(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) {
		return false
	}
	if gerr, ok := errors.AsType[*googleapi.Error](err); ok {
		kind, _ := classify(gerr)
		return kind == KindRateLimited || kind == KindServer
	}
	if _, ok := errors.AsType[*net.OpError](err); ok {
		return true
	}
	if nerr, ok := errors.AsType[net.Error](err); ok && nerr.Timeout() {
		return true
	}
	return errors.Is(err, ErrChecksumMismatch) ||
		errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.EPIPE)
}

func delayFor(err error, attempt int) time.Duration {
	if gerr, ok := errors.AsType[*googleapi.Error](err); ok {
		if ra := gerr.Header.Get("Retry-After"); ra != "" {
			if secs, aerr := strconv.Atoi(ra); aerr == nil {
				return min(time.Duration(secs)*time.Second, maxRetryAfter)
			}
			if t, perr := http.ParseTime(ra); perr == nil {
				return min(time.Until(t), maxRetryAfter)
			}
		}
	}
	d := baseDelay << attempt
	d = d/2 + rand.N(d) // ±50% jitter
	return min(d, maxDelay)
}
