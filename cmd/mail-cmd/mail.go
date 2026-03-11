package mailCmd

import (
	"github.com/spf13/cobra"
	markCmd "github.com/tanq16/gcli/cmd/mail-cmd/mark-cmd"
	"github.com/tanq16/gcli/internal/auth"
	"github.com/tanq16/gcli/internal/mail"
)

func init() {
	MailCmd.AddCommand(markCmd.MarkCmd)
}

var MailCmd = &cobra.Command{
	Use:   "mail",
	Short: "Gmail operations",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		client, err := auth.GetHTTPClient()
		if err != nil {
			return err
		}
		return mail.Init(client)
	},
}
