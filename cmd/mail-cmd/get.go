package mailCmd

import (
	"github.com/spf13/cobra"
	"github.com/tanq16/gdrive/internal/mail"
	u "github.com/tanq16/gdrive/utils"
)

var getCmd = &cobra.Command{
	Use:   "get <message-id>",
	Short: "Show full message (headers + body)",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		msg, err := mail.GetMessage(args[0])
		if err != nil {
			u.PrintFatal("failed to get message", err)
		}

		from := mail.ExtractHeader(msg, "From")
		to := mail.ExtractHeader(msg, "To")
		cc := mail.ExtractHeader(msg, "Cc")
		date := mail.ExtractHeader(msg, "Date")
		subject := mail.ExtractHeader(msg, "Subject")

		headers := []string{"FIELD", "VALUE"}
		rows := [][]string{
			{"From", from},
			{"To", to},
			{"Date", date},
			{"Subject", subject},
		}
		if cc != "" {
			rows = append(rows[:3], append([][]string{{"Cc", cc}}, rows[3:]...)...)
		}

		u.PrintTable(headers, rows)
		u.PrintGeneric("")
		u.PrintGeneric(mail.ExtractBody(msg))
	},
}

func init() {
	MailCmd.AddCommand(getCmd)
}
