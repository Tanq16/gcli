package markCmd

import (
	"github.com/spf13/cobra"
	"github.com/tanq16/gdrive/internal/mail"
	u "github.com/tanq16/gdrive/utils"
)

var unstarCmd = &cobra.Command{
	Use:   "unstar <thread-id>",
	Short: "Remove star from a thread",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := mail.Unstar(args[0]); err != nil {
			u.PrintFatal("failed to unstar message", err)
		}
		u.PrintSuccess("star removed")
	},
}

func init() {
	MarkCmd.AddCommand(unstarCmd)
}
