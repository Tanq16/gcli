package calCmd

import (
	"github.com/spf13/cobra"
	"github.com/tanq16/gdrive/internal/auth"
	"github.com/tanq16/gdrive/internal/cal"
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
		debug, _ := cmd.Root().PersistentFlags().GetBool("debug")
		return cal.Init(client, debug)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}
