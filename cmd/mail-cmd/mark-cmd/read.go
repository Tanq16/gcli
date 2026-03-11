package markCmd

import (
	"github.com/spf13/cobra"
	"github.com/tanq16/gcli/internal/mail"
	u "github.com/tanq16/gcli/utils"
)

var readCmd = &cobra.Command{
	Use:   "read <thread-id>",
	Short: "Mark a thread as read",
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
