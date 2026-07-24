package gapi

import (
	"context"
	"errors"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"strconv"
	"time"

	"google.golang.org/api/googleapi"
)

const (
	maxAttempts   = 4
	baseDelay     = 500 * time.Millisecond
	maxDelay      = 16 * time.Second
	maxRetryAfter = 2 * time.Minute
)

// Retry runs fn up to maxAttempts times, backing off between retryable failures
// and honoring a Retry-After header when present. It always returns the last
// error on exhaustion, and ctx cancellation during a backoff returns ctx.Err().
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

// RetryErr wraps Retry for error-only calls (delete/trash/touch).
func RetryErr(ctx context.Context, fn func() error) error {
	_, err := Retry(ctx, func() (struct{}, error) {
		return struct{}{}, fn()
	})
	return err
}

func retryable(err error) bool {
	if gerr, ok := errors.AsType[*googleapi.Error](err); ok {
		kind, _ := classify(gerr)
		return kind == KindRateLimited || kind == KindServer
	}
	var nerr net.Error
	if errors.As(err, &nerr) && nerr.Timeout() {
		return true
	}
	return errors.Is(err, io.ErrUnexpectedEOF)
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
