package mail

import (
	"context"
	"fmt"
	"net/http"

	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

// Service is the authenticated Gmail service
var Service *gmail.Service

// Init creates a Gmail service from an authenticated HTTP client
func Init(client *http.Client) error {
	srv, err := gmail.NewService(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		return fmt.Errorf("failed to create Gmail service: %w", err)
	}
	Service = srv
	return nil
}
