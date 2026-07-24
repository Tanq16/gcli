package gapi

import (
	"errors"
	"fmt"

	u "github.com/tanq16/gcli/utils"
	"google.golang.org/api/googleapi"
)

type Kind int

const (
	KindGeneric Kind = iota
	KindAuth
	KindNotFound
	KindPermission
	KindRateLimited
	KindServer
)

// APIError is the single normalized error type for the package. It preserves the
// original *googleapi.Error via Unwrap so callers keep the full chain, and maps
// its Kind to the process exit code.
type APIError struct {
	Kind Kind
	Msg  string
	Err  error
}

func (e *APIError) Error() string { return e.Msg }
func (e *APIError) Unwrap() error { return e.Err }

func (e *APIError) ExitCode() int {
	switch e.Kind {
	case KindAuth:
		return u.ExitAuth
	case KindNotFound:
		return u.ExitNotFound
	case KindPermission:
		return u.ExitPermission
	case KindRateLimited:
		return u.ExitRateLimited
	default:
		return u.ExitGeneric
	}
}

func classify(gerr *googleapi.Error) (Kind, string) {
	switch {
	case gerr.Code == 401:
		return KindAuth, "authentication failed — run 'gcli login'"
	case gerr.Code == 404:
		return KindNotFound, "not found"
	case gerr.Code == 429:
		return KindRateLimited, "rate limited"
	case gerr.Code == 403:
		for _, e := range gerr.Errors {
			if e.Reason == "rateLimitExceeded" || e.Reason == "userRateLimitExceeded" {
				return KindRateLimited, "rate limited"
			}
		}
		return KindPermission, "permission denied"
	case gerr.Code == 408 || gerr.Code >= 500:
		return KindServer, "server error — try again later"
	default:
		return KindGeneric, fmt.Sprintf("API error %d", gerr.Code)
	}
}

func HandleError(err error) error {
	if err == nil {
		return nil
	}
	gerr, ok := errors.AsType[*googleapi.Error](err)
	if !ok {
		return err
	}
	kind, msg := classify(gerr)
	return &APIError{Kind: kind, Msg: msg, Err: gerr}
}
