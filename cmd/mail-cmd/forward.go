package mailCmd

import (
	"github.com/spf13/cobra"
	"github.com/tanq16/gdrive/internal/mail"
	u "github.com/tanq16/gdrive/utils"
)

var forwardFlags struct {
	to       []string
	bodyFile string
}

var forwardCmd = &cobra.Command{
	Use:   "forward <message-id>",
	Short: "Forward a message",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		note, contentType := resolveBody(forwardFlags.bodyFile, false)

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
	forwardCmd.MarkFlagRequired("to")
}
