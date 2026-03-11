package markCmd

import (
	"github.com/spf13/cobra"
	"github.com/tanq16/gdrive/internal/mail"
	u "github.com/tanq16/gdrive/utils"
)

var trashCmd = &cobra.Command{
	Use:   "trash <message-id>",
	Short: "Move a message to trash",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := mail.Trash(args[0]); err != nil {
			u.PrintFatal("failed to trash message", err)
		}
		u.PrintSuccess("message moved to trash")
	},
}

func init() {
	MarkCmd.AddCommand(trashCmd)
}
