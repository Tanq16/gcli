package mailCmd

import (
	"github.com/spf13/cobra"
	"github.com/tanq16/gdrive/internal/mail"
	u "github.com/tanq16/gdrive/utils"
)

var replyFlags struct {
	all       bool
	attach    []string
	bodyFile  string
	signature string
}

var replyCmd = &cobra.Command{
	Use:   "reply <thread-id>",
	Short: "Reply to the last message in a thread",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		body, contentType := resolveBody(replyFlags.bodyFile, true)
		body, contentType = applySignature(body, contentType, replyFlags.signature)

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
	replyCmd.Flags().StringVar(&replyFlags.signature, "signature", "default", "Gmail signature to append (\"default\", email alias, or \"none\")")
}
