package markCmd

import (
	"github.com/spf13/cobra"
	"github.com/tanq16/gcli/internal/mail"
	u "github.com/tanq16/gcli/utils"
)

var archiveCmd = &cobra.Command{
	Use:   "archive <thread-id>",
	Short: "Archive a thread (remove from inbox)",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := mail.Archive(args[0]); err != nil {
			u.PrintFatal("failed to archive message", err)
		}
		u.PrintSuccess("message archived")
	},
}

func init() {
	MarkCmd.AddCommand(archiveCmd)
}
