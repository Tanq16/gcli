package markCmd

import (
	"github.com/spf13/cobra"
	"github.com/tanq16/gdrive/internal/mail"
	u "github.com/tanq16/gdrive/utils"
)

var starCmd = &cobra.Command{
	Use:   "star <thread-id>",
	Short: "Star a thread",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := mail.Star(args[0]); err != nil {
			u.PrintFatal("failed to star message", err)
		}
		u.PrintSuccess("message starred")
	},
}

func init() {
	MarkCmd.AddCommand(starCmd)
}
