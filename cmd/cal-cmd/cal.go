package calCmd

import (
	"github.com/spf13/cobra"
	"github.com/tanq16/gcli/internal/auth"
	"github.com/tanq16/gcli/internal/cal"
)

// CalCmd is the parent command for all Google Calendar operations
var CalCmd = &cobra.Command{
	Use:   "cal",
	Short: "Google Calendar operations",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		client, err := auth.GetHTTPClient()
		if err != nil {
			return err
		}
		return cal.Init(client)
	},
}
