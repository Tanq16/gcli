package cmd

import (
	"errors"

	"github.com/spf13/cobra"
	"github.com/tanq16/gcli/internal/auth"
	u "github.com/tanq16/gcli/utils"
)

var loginFlags struct {
	setup        bool
	clientID     string
	clientSecret string
	overwrite    bool
}

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with Google services",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if !loginFlags.setup && (loginFlags.clientID != "" || loginFlags.clientSecret != "" || loginFlags.overwrite) {
			u.PrintFatalCode("--client-id/--client-secret/--overwrite require --setup", nil, u.ExitUsage)
		}

		if loginFlags.setup {
			err := auth.RunSetup(cmd.Context(), auth.SetupOptions{
				ClientID:     loginFlags.clientID,
				ClientSecret: loginFlags.clientSecret,
				Overwrite:    loginFlags.overwrite,
			})
			if errors.Is(err, auth.ErrAborted) {
				u.PrintInfo("setup cancelled")
				return
			}
			if err != nil {
				u.PrintFatal("setup failed", err)
			}
			return
		}

		// The PKCE verifier is minted in this process, so a code piped into a later invocation can never match it.
		if u.GlobalForAIFlag {
			u.PrintFatalCode("login requires an interactive terminal — run 'gcli login' without --for-ai", nil, u.ExitUsage)
		}

		config, _, err := auth.LoadCredentials()
		if errors.Is(err, auth.ErrNoCredentials) {
			u.PrintFatalCode("", auth.WithSetupHint(err), u.ExitAuth)
		}
		if err != nil {
			u.PrintFatalCode("failed to load credentials", err, u.ExitAuth)
		}
		if _, err := auth.Login(cmd.Context(), config); err != nil {
			u.PrintFatalCode("login failed", err, u.ExitAuth)
		}
		u.PrintSuccess("authenticated successfully - token saved")
	},
}

func init() {
	rootCmd.AddCommand(loginCmd)

	loginCmd.Flags().BoolVar(&loginFlags.setup, "setup", false, "Run the guided credential-setup wizard")
	loginCmd.Flags().StringVar(&loginFlags.clientID, "client-id", "", "(setup) OAuth client ID")
	loginCmd.Flags().StringVar(&loginFlags.clientSecret, "client-secret", "", "(setup) OAuth client secret")
	loginCmd.Flags().BoolVar(&loginFlags.overwrite, "overwrite", false, "(setup) replace existing credentials.json without prompting")
}
