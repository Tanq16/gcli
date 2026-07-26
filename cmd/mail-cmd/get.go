package mailCmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tanq16/gcli/internal/mail"
	u "github.com/tanq16/gcli/utils"
	"google.golang.org/api/gmail/v1"
)

var getFlags struct {
	withQuote bool
}

var getCmd = &cobra.Command{
	Use:     "get <thread-id>",
	Aliases: []string{"view"},
	Short:   "Show all messages in a thread",
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		thread, err := mail.GetThread(args[0])
		if err != nil {
			u.PrintFatal("failed to get thread", err)
		}
		msgs := thread.Messages

		if u.GlobalForAIFlag {
			printThreadForAI(msgs)
			return
		}
		printThreadHuman(msgs)
	},
}

func init() {
	MailCmd.AddCommand(getCmd)
	getCmd.Flags().BoolVar(&getFlags.withQuote, "with-quote", false, "Include quoted/block-quoted text in output")
}

func messageBody(msg *gmail.Message) string {
	body := mail.ExtractBody(msg)
	if !getFlags.withQuote {
		body = mail.StripQuotedText(body)
	}
	return body
}

func printThreadHuman(msgs []*gmail.Message) {
	for i, msg := range msgs {
		u.PrintGeneric(fmt.Sprintf("--- Message %d of %d ---", i+1, len(msgs)))
		from := mail.ExtractHeader(msg, "From")
		date := mail.ExtractHeader(msg, "Date")
		u.PrintGeneric(fmt.Sprintf("From: %s  |  Date: %s", from, date))
		u.PrintGeneric("")
		u.PrintGeneric(messageBody(msg))
		u.PrintGeneric("")
	}
}

func printThreadForAI(msgs []*gmail.Message) {
	rows := make([][]string, 0, len(msgs))
	for _, msg := range msgs {
		rows = append(rows, []string{
			msg.Id,
			mail.ExtractHeader(msg, "From"),
			mail.ExtractHeader(msg, "Date"),
			mail.ExtractHeader(msg, "Subject"),
		})
	}
	u.PrintTable([]string{"MSG", "FROM", "DATE", "SUBJECT"}, rows)
	for _, msg := range msgs {
		u.PrintGeneric(fmt.Sprintf("[BODY %s START]", msg.Id))
		u.PrintGeneric(messageBody(msg))
		u.PrintGeneric(fmt.Sprintf("[BODY %s END]", msg.Id))
	}
}
