package mailCmd

import (
	"github.com/spf13/cobra"
	"github.com/tanq16/gdrive/internal/mail"
	u "github.com/tanq16/gdrive/utils"
)

var searchFlags struct {
	max int64
}

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search messages using Gmail search syntax",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		messages, err := mail.SearchMessages(args[0], searchFlags.max)
		if err != nil {
			u.PrintFatal("search failed", err)
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
	MailCmd.AddCommand(searchCmd)
	searchCmd.Flags().Int64Var(&searchFlags.max, "max", 20, "Maximum number of results")
}
