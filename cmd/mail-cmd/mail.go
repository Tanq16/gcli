package mailCmd

import (
	"github.com/spf13/cobra"
	"github.com/tanq16/gcli/internal/auth"
	"github.com/tanq16/gcli/internal/mail"
	u "github.com/tanq16/gcli/utils"
)

var MailCmd = &cobra.Command{
	Use:   "mail",
	Short: "Gmail operations",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		client, err := auth.GetHTTPClient(cmd.Context())
		if err != nil {
			u.PrintFatalCode("not authenticated — run 'gcli login'", err, u.ExitAuth)
		}
		if err := mail.Init(client); err != nil {
			u.PrintFatalCode("failed to initialize Gmail client", err, u.ExitAuth)
		}
	},
}
