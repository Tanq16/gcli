package mailCmd

import (
	"fmt"
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
	Short: "List recent threads (default: 20)",
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

		threads, err := mail.ListThreads(listFlags.label, listFlags.unread, count)
		if err != nil {
			u.PrintFatal("failed to list threads", err)
		}

		if len(threads) == 0 {
			u.PrintInfo("no threads found")
			return
		}

		headers := []string{"ID", "FROM", "SUBJECT", "DATE"}
		var rows [][]string
		for _, t := range threads {
			from := truncateString(t.From, 30)
			subject := t.Subject
			if t.MessageCount > 1 {
				subject = fmt.Sprintf("[%d] %s", t.MessageCount, subject)
			}
			if t.Unread {
				subject = "* " + subject
			}
			subject = truncateString(subject, 80)
			rows = append(rows, []string{t.ID, from, subject, t.Date})
		}

		u.PrintTable(headers, rows)
	},
}

func init() {
	MailCmd.AddCommand(listCmd)
	listCmd.Flags().StringVar(&listFlags.label, "label", "INBOX", "Label to list threads from")
	listCmd.Flags().BoolVar(&listFlags.unread, "unread", false, "Only show unread threads")
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
