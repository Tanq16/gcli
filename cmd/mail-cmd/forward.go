package mailCmd

import (
	"github.com/spf13/cobra"
	"github.com/tanq16/gcli/internal/mail"
	u "github.com/tanq16/gcli/utils"
)

var forwardFlags struct {
	to        []string
	attach    []string
	bodyFile  string
	signature string
	draft     bool
}

var forwardCmd = &cobra.Command{
	Use:   "forward <thread-id>",
	Short: "Forward the last message in a thread (re-attaches original attachments)",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		atts, err := mail.LoadAttachments(forwardFlags.attach)
		if err != nil {
			u.PrintFatal("failed to read attachment", err)
		}
		note, contentType := resolveBody(forwardFlags.bodyFile, false)
		note, contentType = applySignature(note, contentType, forwardFlags.signature)

		opts, err := mail.BuildForwardOptions(args[0], forwardFlags.to, note, contentType, atts)
		if err != nil {
			u.PrintFatal("failed to build forward", err)
		}
		sendOrDraft(opts, forwardFlags.draft)
	},
}

func init() {
	MailCmd.AddCommand(forwardCmd)
	forwardCmd.Flags().StringArrayVarP(&forwardFlags.to, "to", "t", nil, "Recipient email address (repeatable)")
	forwardCmd.Flags().StringArrayVarP(&forwardFlags.attach, "attach", "a", nil, "Additional file attachment path (repeatable)")
	forwardCmd.Flags().StringVarP(&forwardFlags.bodyFile, "body-file", "f", "", "Read optional note from file")
	forwardCmd.Flags().StringVar(&forwardFlags.signature, "signature", "default", "Gmail signature to append (\"default\", email alias, or \"none\")")
	forwardCmd.Flags().BoolVar(&forwardFlags.draft, "draft", false, "Create a draft instead of sending")
	forwardCmd.MarkFlagRequired("to")
}
