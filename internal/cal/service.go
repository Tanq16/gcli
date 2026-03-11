package cal

import (
	"context"
	"fmt"
	"net/http"

	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
)

// Service is the authenticated Calendar service
var Service *calendar.Service

// Debug indicates whether debug logging is enabled
var Debug bool

// Init creates a Calendar service from an authenticated HTTP client
func Init(client *http.Client, debug bool) error {
	srv, err := calendar.NewService(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		return fmt.Errorf("failed to create Calendar service: %w", err)
	}
	Service = srv
	Debug = debug
	return nil
}

// HandleError formats a Google API error into a user-friendly message
func HandleError(err error) error {
	if err == nil {
		return nil
	}
	gerr, ok := err.(*googleapi.Error)
	if !ok {
		return err
	}
	switch gerr.Code {
	case 404:
		return fmt.Errorf("not found")
	case 403:
		for _, e := range gerr.Errors {
			if e.Reason == "userRateLimitExceeded" || e.Reason == "rateLimitExceeded" {
				return fmt.Errorf("rate limited — wait a moment and try again")
			}
		}
		return fmt.Errorf("permission denied")
	case 429:
		return fmt.Errorf("rate limited — wait a moment and try again")
	case 500, 503:
		return fmt.Errorf("server error — try again later")
	default:
		return fmt.Errorf("API error %d: %s", gerr.Code, gerr.Message)
	}
}
