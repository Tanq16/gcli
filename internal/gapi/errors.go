package gapi

import (
	"fmt"

	"google.golang.org/api/googleapi"
)

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
