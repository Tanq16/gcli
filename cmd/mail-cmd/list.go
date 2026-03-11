package mailCmd

import (
	"strconv"

	"github.com/spf13/cobra"
	"github.com/tanq16/gdrive/internal/mail"
	u "github.com/tanq16/gdrive/utils"
)

var listFlags struct {
	label  string
	unread bool
}

var listCmd = &cobra.Command{
	Use:   "list [count]",
	Short: "List recent messages (default: 20)",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		count := int64(20)
		if len(args) > 0 {
			n, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				u.PrintFatal("invalid count", err)
			}
			count = n
		}

		messages, err := mail.ListMessages(listFlags.label, listFlags.unread, count)
		if err != nil {
			u.PrintFatal("failed to list messages", err)
		}

		if len(messages) == 0 {
			u.PrintInfo("no messages found")
			return
		}

		headers := []string{"STATUS", "FROM", "SUBJECT", "DATE", "ID"}
		var rows [][]string
		for _, m := range messages {
			status := " "
			if m.Unread {
				status = "*"
			}
			from := mail.TruncateString(m.From, 25)
			subject := mail.TruncateString(m.Subject, 50)
			rows = append(rows, []string{status, from, subject, m.Date, m.ID})
		}

		u.PrintTable(headers, rows)
	},
}

func init() {
	MailCmd.AddCommand(listCmd)
	listCmd.Flags().StringVar(&listFlags.label, "label", "INBOX", "Label to list messages from")
	listCmd.Flags().BoolVar(&listFlags.unread, "unread", false, "Only show unread messages")
}
