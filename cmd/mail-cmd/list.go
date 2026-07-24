package mailCmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tanq16/gcli/internal/mail"
	u "github.com/tanq16/gcli/utils"
)

var listFlags struct {
	label  string
	unread bool
	limit  int64
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List recent threads",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		threads, err := mail.ListThreads(cmd.Context(), listFlags.label, listFlags.unread, listFlags.limit)
		if err != nil {
			u.PrintFatal("failed to list threads", err)
		}

		if len(threads) == 0 {
			u.PrintInfo("no threads found")
			return
		}

		u.PrintTable([]string{"ID", "FROM", "SUBJECT", "DATE"}, threadRows(threads))
	},
}

func init() {
	MailCmd.AddCommand(listCmd)
	listCmd.Flags().StringVar(&listFlags.label, "label", "INBOX", "Label to list threads from")
	listCmd.Flags().BoolVar(&listFlags.unread, "unread", false, "Only show unread threads")
	listCmd.Flags().Int64VarP(&listFlags.limit, "limit", "n", 20, "Maximum number of threads")
}

func threadRows(threads []mail.ThreadSummary) [][]string {
	rows := make([][]string, 0, len(threads))
	for _, t := range threads {
		subject := t.Subject
		if t.MessageCount > 1 {
			subject = fmt.Sprintf("[%d] %s", t.MessageCount, subject)
		}
		if t.Unread {
			subject = "* " + subject
		}
		rows = append(rows, []string{t.ID, t.From, subject, t.Date})
	}
	return rows
}
