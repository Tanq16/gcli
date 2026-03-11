package markCmd

import (
	"github.com/spf13/cobra"
	"github.com/tanq16/gdrive/internal/mail"
	u "github.com/tanq16/gdrive/utils"
)

var unreadCmd = &cobra.Command{
	Use:   "unread <thread-id>",
	Short: "Mark a thread as unread",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := mail.MarkUnread(args[0]); err != nil {
			u.PrintFatal("failed to mark as unread", err)
		}
		u.PrintSuccess("marked as unread")
	},
}

func init() {
	MarkCmd.AddCommand(unreadCmd)
}
