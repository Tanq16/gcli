package mailCmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tanq16/gcli/internal/mail"
	u "github.com/tanq16/gcli/utils"
)

var markCmd = &cobra.Command{
	Use:   "mark <verb> <thread-id>",
	Short: "Mark a thread (read unread star unstar archive trash spam)",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		verb, id := args[0], args[1]
		var err error
		var ok string
		switch verb {
		case "read":
			err, ok = mail.MarkRead(id), "marked as read"
		case "unread":
			err, ok = mail.MarkUnread(id), "marked as unread"
		case "star":
			err, ok = mail.Star(id), "starred"
		case "unstar":
			err, ok = mail.Unstar(id), "unstarred"
		case "archive":
			err, ok = mail.Archive(id), "archived"
		case "trash":
			err, ok = mail.Trash(id), "trashed"
		case "spam":
			err, ok = mail.MarkSpam(id), "marked as spam"
		default:
			u.PrintFatalCode(fmt.Sprintf("unknown verb %q (read unread star unstar archive trash spam)", verb), nil, u.ExitUsage)
		}
		if err != nil {
			u.PrintFatal("failed to mark thread", err)
		}
		u.PrintSuccess(ok)
	},
}

func init() {
	MailCmd.AddCommand(markCmd)
}
