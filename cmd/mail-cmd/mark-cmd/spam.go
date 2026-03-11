package markCmd

import (
	"github.com/spf13/cobra"
	"github.com/tanq16/gcli/internal/mail"
	u "github.com/tanq16/gcli/utils"
)

var spamCmd = &cobra.Command{
	Use:   "spam <thread-id>",
	Short: "Mark a thread as spam",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := mail.MarkSpam(args[0]); err != nil {
			u.PrintFatal("failed to mark as spam", err)
		}
		u.PrintSuccess("marked as spam")
	},
}

func init() {
	MarkCmd.AddCommand(spamCmd)
}
