package mailCmd

import (
	"github.com/spf13/cobra"
	"github.com/tanq16/gcli/internal/mail"
	u "github.com/tanq16/gcli/utils"
)

var forwardFlags struct {
	to        []string
	bodyFile  string
	signature string
}

var forwardCmd = &cobra.Command{
	Use:   "forward <thread-id>",
	Short: "Forward the last message in a thread",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		note, contentType := resolveBody(forwardFlags.bodyFile, false)
		note, contentType = applySignature(note, contentType, forwardFlags.signature)

		if err := mail.ForwardMessage(args[0], forwardFlags.to, note, contentType); err != nil {
			u.PrintFatal("failed to forward message", err)
		}
		u.PrintSuccess("message forwarded")
	},
}

func init() {
	MailCmd.AddCommand(forwardCmd)
	forwardCmd.Flags().StringArrayVarP(&forwardFlags.to, "to", "t", nil, "Recipient email address (repeatable)")
	forwardCmd.Flags().StringVar(&forwardFlags.bodyFile, "body-file", "", "Read optional note from file")
	forwardCmd.Flags().StringVar(&forwardFlags.signature, "signature", "default", "Gmail signature to append (\"default\", email alias, or \"none\")")
	forwardCmd.MarkFlagRequired("to")
}
