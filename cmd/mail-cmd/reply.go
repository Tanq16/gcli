package mailCmd

import (
	"github.com/spf13/cobra"
	"github.com/tanq16/gdrive/internal/mail"
	u "github.com/tanq16/gdrive/utils"
)

var replyFlags struct {
	all      bool
	attach   []string
	bodyFile string
}

var replyCmd = &cobra.Command{
	Use:   "reply <message-id>",
	Short: "Reply to a message",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		body, contentType := resolveBody(replyFlags.bodyFile, true)

		if err := mail.ReplyMessage(args[0], body, contentType, replyFlags.all, replyFlags.attach); err != nil {
			u.PrintFatal("failed to send reply", err)
		}
		u.PrintSuccess("reply sent")
	},
}

func init() {
	MailCmd.AddCommand(replyCmd)
	replyCmd.Flags().BoolVar(&replyFlags.all, "all", false, "Reply to all recipients")
	replyCmd.Flags().StringArrayVarP(&replyFlags.attach, "attach", "A", nil, "File attachment path (repeatable)")
	replyCmd.Flags().StringVar(&replyFlags.bodyFile, "body-file", "", "Read body from file (.txt, .html, .md)")
}
