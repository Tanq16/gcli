package mailCmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tanq16/gcli/internal/mail"
	u "github.com/tanq16/gcli/utils"
)

var getFlags struct {
	withQuote bool
}

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
			body := mail.ExtractBody(msg)
			if !getFlags.withQuote {
				body = stripQuotedText(body)
			}
			u.PrintGeneric(body)
			u.PrintGeneric("")
		}
	},
}

func init() {
	MailCmd.AddCommand(getCmd)
	getCmd.Flags().BoolVar(&getFlags.withQuote, "with-quote", false, "Include quoted/block-quoted text in output")
}

func stripQuotedText(body string) string {
	lines := strings.Split(body, "\n")
	var result []string
	for _, line := range lines {
		if strings.HasPrefix(line, ">") || strings.HasPrefix(line, "&gt;") {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "On ") && strings.Contains(trimmed, " wrote:") {
			break
		}
		if strings.HasPrefix(trimmed, "________") {
			break
		}
		result = append(result, line)
	}
	return strings.TrimRight(strings.Join(result, "\n"), "\n ")
}
