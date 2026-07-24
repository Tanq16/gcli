package gapi

import (
	"context"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

	u "github.com/tanq16/gcli/utils"
	"google.golang.org/api/googleapi"
)

func gerr(code int, reason string) *googleapi.Error {
	e := &googleapi.Error{Code: code}
	if reason != "" {
		e.Errors = []googleapi.ErrorItem{{Reason: reason}}
	}
	return e
}

func TestClassify(t *testing.T) {
	tests := []struct {
		name     string
		err      *googleapi.Error
		wantKind Kind
	}{
		{"401 auth", gerr(401, ""), KindAuth},
		{"404 not found", gerr(404, ""), KindNotFound},
		{"429 rate", gerr(429, ""), KindRateLimited},
		{"403 rate", gerr(403, "rateLimitExceeded"), KindRateLimited},
		{"403 user rate", gerr(403, "userRateLimitExceeded"), KindRateLimited},
		{"403 permission", gerr(403, "insufficientPermissions"), KindPermission},
		{"403 no reason", gerr(403, ""), KindPermission},
		{"408 server", gerr(408, ""), KindServer},
		{"500 server", gerr(500, ""), KindServer},
		{"502 server", gerr(502, ""), KindServer},
		{"503 server", gerr(503, ""), KindServer},
		{"418 generic", gerr(418, ""), KindGeneric},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, _ := classify(tt.err); got != tt.wantKind {
				t.Fatalf("classify(%d) = %v, want %v", tt.err.Code, got, tt.wantKind)
			}
		})
	}
}

func TestHandleError(t *testing.T) {
	t.Run("nil passes through", func(t *testing.T) {
		if HandleError(nil) != nil {
			t.Fatal("expected nil")
		}
	})

	t.Run("non-googleapi returned as-is", func(t *testing.T) {
		orig := errors.New("plain")
		if got := HandleError(orig); got != orig {
			t.Fatalf("expected original error, got %v", got)
		}
	})

	t.Run("wraps and preserves original", func(t *testing.T) {
		orig := gerr(404, "")
		got := HandleError(orig)
		var api *APIError
		if !errors.As(got, &api) {
			t.Fatal("expected *APIError")
		}
		if api.Kind != KindNotFound {
			t.Fatalf("kind = %v, want KindNotFound", api.Kind)
		}
		var recovered *googleapi.Error
		if !errors.As(got, &recovered) || recovered != orig {
			t.Fatal("expected original *googleapi.Error recoverable through chain")
		}
	})
}

func TestExitCode(t *testing.T) {
	tests := []struct {
		kind Kind
		want int
	}{
		{KindAuth, u.ExitAuth},
		{KindNotFound, u.ExitNotFound},
		{KindPermission, u.ExitPermission},
		{KindRateLimited, u.ExitRateLimited},
		{KindServer, u.ExitGeneric},
		{KindGeneric, u.ExitGeneric},
	}
	for _, tt := range tests {
		e := &APIError{Kind: tt.kind}
		if got := e.ExitCode(); got != tt.want {
			t.Fatalf("ExitCode(%v) = %d, want %d", tt.kind, got, tt.want)
		}
	}
}

type timeoutErr struct{}

func (timeoutErr) Error() string   { return "timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return true }

func TestRetryable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"429", gerr(429, ""), true},
		{"403 rate", gerr(403, "rateLimitExceeded"), true},
		{"403 permission", gerr(403, "insufficientPermissions"), false},
		{"404", gerr(404, ""), false},
		{"500", gerr(500, ""), true},
		{"502", gerr(502, ""), true},
		{"net timeout", timeoutErr{}, true},
		{"unexpected eof", io.ErrUnexpectedEOF, true},
		{"plain", errors.New("nope"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := retryable(tt.err); got != tt.want {
				t.Fatalf("retryable(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func withRetryAfter(code int, value string) *googleapi.Error {
	return &googleapi.Error{Code: code, Header: http.Header{"Retry-After": []string{value}}}
}

func TestDelayFor(t *testing.T) {
	t.Run("retry-after seconds", func(t *testing.T) {
		if d := delayFor(withRetryAfter(429, "3"), 0); d != 3*time.Second {
			t.Fatalf("delayFor seconds = %v, want 3s", d)
		}
	})

	t.Run("retry-after capped", func(t *testing.T) {
		if d := delayFor(withRetryAfter(429, "9999"), 0); d != maxRetryAfter {
			t.Fatalf("delayFor cap = %v, want %v", d, maxRetryAfter)
		}
	})

	t.Run("retry-after http date", func(t *testing.T) {
		future := time.Now().Add(5 * time.Second).UTC().Format(http.TimeFormat)
		d := delayFor(withRetryAfter(429, future), 0)
		if d < 3*time.Second || d > 7*time.Second {
			t.Fatalf("delayFor date = %v, want ~5s", d)
		}
	})

	t.Run("backoff jitter bounds", func(t *testing.T) {
		for range 50 {
			d := delayFor(errors.New("x"), 0)
			if d < baseDelay/2 || d >= baseDelay+baseDelay/2 {
				t.Fatalf("delayFor attempt 0 = %v, out of jitter bounds", d)
			}
		}
	})

	t.Run("backoff capped at maxDelay", func(t *testing.T) {
		if d := delayFor(errors.New("x"), 10); d > maxDelay {
			t.Fatalf("delayFor high attempt = %v, exceeds maxDelay", d)
		}
	})
}

func TestRetry(t *testing.T) {
	t.Run("success first try", func(t *testing.T) {
		calls := 0
		v, err := Retry(t.Context(), func() (int, error) {
			calls++
			return 42, nil
		})
		if err != nil || v != 42 || calls != 1 {
			t.Fatalf("v=%d err=%v calls=%d", v, err, calls)
		}
	})

	t.Run("exhausts and returns last error", func(t *testing.T) {
		calls := 0
		last := withRetryAfter(429, "0")
		_, err := Retry(t.Context(), func() (int, error) {
			calls++
			return 0, last
		})
		if calls != maxAttempts {
			t.Fatalf("calls = %d, want %d", calls, maxAttempts)
		}
		if !errors.Is(err, last) {
			t.Fatalf("err = %v, want last error", err)
		}
	})

	t.Run("non-retryable stops immediately", func(t *testing.T) {
		calls := 0
		_, err := Retry(t.Context(), func() (int, error) {
			calls++
			return 0, gerr(404, "")
		})
		if calls != 1 || err == nil {
			t.Fatalf("calls = %d err = %v", calls, err)
		}
	})

	t.Run("cancelled context during backoff", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		_, err := Retry(ctx, func() (int, error) {
			return 0, gerr(500, "")
		})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", err)
		}
	})
}
