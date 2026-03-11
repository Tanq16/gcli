package markCmd

import (
	"github.com/spf13/cobra"
	"github.com/tanq16/gdrive/internal/mail"
	u "github.com/tanq16/gdrive/utils"
)

var readCmd = &cobra.Command{
	Use:   "read <message-id>",
	Short: "Mark a message as read",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := mail.MarkRead(args[0]); err != nil {
			u.PrintFatal("failed to mark as read", err)
		}
		u.PrintSuccess("marked as read")
	},
}

func init() {
	MarkCmd.AddCommand(readCmd)
}
