package mailCmd

import (
	"errors"

	"github.com/spf13/cobra"
	"github.com/tanq16/gcli/internal/auth"
	"github.com/tanq16/gcli/internal/mail"
	u "github.com/tanq16/gcli/utils"
)

var MailCmd = &cobra.Command{
	Use:   "mail",
	Short: "Gmail operations",
	// Runnable so cobra reaches ValidateArgs; a bare parent returns ErrHelp first and a
	// mistyped subcommand would print help and exit 0.
	Args: cobra.NoArgs,
	Run:  func(cmd *cobra.Command, args []string) { _ = cmd.Help() },
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// A parent invocation only prints help, which must work unauthenticated.
		if cmd.HasSubCommands() {
			return
		}
		client, err := auth.GetHTTPClient(cmd.Context())
		if errors.Is(err, auth.ErrNoCredentials) {
			u.PrintFatalCode(err.Error()+"; "+auth.NoCredentialsHint, nil, u.ExitAuth)
		}
		if err != nil {
			u.PrintFatalCode("not authenticated — run 'gcli login'", err, u.ExitAuth)
		}
		if err := mail.Init(client); err != nil {
			u.PrintFatalCode("failed to initialize Gmail client", err, u.ExitAuth)
		}
	},
}
