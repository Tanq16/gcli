package mailCmd

import (
	"github.com/spf13/cobra"
	"github.com/tanq16/gcli/internal/mail"
	u "github.com/tanq16/gcli/utils"
)

var replyFlags struct {
	all       bool
	attach    []string
	bodyFile  string
	signature string
	draft     bool
}

var replyCmd = &cobra.Command{
	Use:   "reply <thread-id>",
	Short: "Reply to the last message in a thread",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		atts, err := mail.LoadAttachments(replyFlags.attach)
		if err != nil {
			u.PrintFatal("failed to read attachment", err)
		}
		body, contentType := resolveBody(replyFlags.bodyFile, true)
		body, contentType = applySignature(body, contentType, replyFlags.signature)

		opts, err := mail.BuildReplyOptions(args[0], body, contentType, replyFlags.all, atts)
		if err != nil {
			u.PrintFatal("failed to build reply", err)
		}
		sendOrDraft(opts, replyFlags.draft)
	},
}

func init() {
	MailCmd.AddCommand(replyCmd)
	replyCmd.Flags().BoolVar(&replyFlags.all, "all", false, "Reply to all recipients")
	replyCmd.Flags().StringArrayVarP(&replyFlags.attach, "attach", "a", nil, "File attachment path (repeatable)")
	replyCmd.Flags().StringVarP(&replyFlags.bodyFile, "body-file", "f", "", "Read body from file (.txt, .html)")
	replyCmd.Flags().StringVar(&replyFlags.signature, "signature", "default", "Gmail signature to append (\"default\", email alias, or \"none\")")
	replyCmd.Flags().BoolVar(&replyFlags.draft, "draft", false, "Create a draft instead of sending")
}
