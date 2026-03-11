package mailCmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tanq16/gcli/internal/mail"
	u "github.com/tanq16/gcli/utils"
)

var searchFlags struct {
	max int64
}

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search threads using Gmail search syntax",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		threads, err := mail.SearchThreads(context.Background(), args[0], searchFlags.max)
		if err != nil {
			u.PrintFatal("search failed", err)
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
	MailCmd.AddCommand(searchCmd)
	searchCmd.Flags().Int64Var(&searchFlags.max, "max", 20, "Maximum number of results")
}
