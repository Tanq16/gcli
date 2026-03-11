package calCmd

import (
	"github.com/spf13/cobra"
)

// CalCmd is the parent command for all Google Calendar operations
var CalCmd = &cobra.Command{
	Use:   "cal",
	Short: "Google Calendar operations",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}
