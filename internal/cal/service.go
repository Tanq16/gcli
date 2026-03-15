package cal

import (
	"context"
	"fmt"
	"net/http"

	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// Service is the authenticated Calendar service
var Service *calendar.Service

// Init creates a Calendar service from an authenticated HTTP client
func Init(client *http.Client) error {
	srv, err := calendar.NewService(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		return fmt.Errorf("failed to create Calendar service: %w", err)
	}
	Service = srv
	return nil
}

