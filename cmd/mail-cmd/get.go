package mailCmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tanq16/gdrive/internal/mail"
	u "github.com/tanq16/gdrive/utils"
)

var getCmd = &cobra.Command{
	Use:   "get <thread-id>",
	Short: "Show all messages in a thread",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		thread, err := mail.GetThread(args[0])
		if err != nil {
			u.PrintFatal("failed to get thread", err)
		}

		msgs := thread.Messages
		for i, msg := range msgs {
			u.PrintGeneric(fmt.Sprintf("--- Message %d of %d ---", i+1, len(msgs)))
			from := mail.ExtractHeader(msg, "From")
			date := mail.ExtractHeader(msg, "Date")
			u.PrintGeneric(fmt.Sprintf("From: %s  |  Date: %s", from, date))
			u.PrintGeneric("")
			u.PrintGeneric(mail.ExtractBody(msg))
			u.PrintGeneric("")
		}
	},
}

func init() {
	MailCmd.AddCommand(getCmd)
}
