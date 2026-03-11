package mailCmd

import (
	"github.com/spf13/cobra"
)

// MailCmd is the parent command for all Gmail operations
var MailCmd = &cobra.Command{
	Use:   "mail",
	Short: "Gmail operations",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}
