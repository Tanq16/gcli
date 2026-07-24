package cmd

import (
	"errors"

	"github.com/spf13/cobra"
	"github.com/tanq16/gcli/internal/auth"
	u "github.com/tanq16/gcli/utils"
)

var loginFlags struct {
	manual       bool
	setup        bool
	projectID    string
	clientID     string
	clientSecret string
	overwrite    bool
}

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with Google services",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if !loginFlags.setup && (loginFlags.projectID != "" || loginFlags.clientID != "" || loginFlags.clientSecret != "" || loginFlags.overwrite) {
			u.PrintFatalCode("--project-id/--client-id/--client-secret/--overwrite require --setup", nil, u.ExitUsage)
		}

		if loginFlags.setup {
			err := auth.RunSetup(cmd.Context(), auth.SetupOptions{
				ProjectID:    loginFlags.projectID,
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

		config, _, err := auth.LoadCredentials()
		if err != nil {
			u.PrintFatalCode("failed to load credentials", err, u.ExitAuth)
		}
		mode := "default"
		if loginFlags.manual {
			mode = "manual"
		}
		if _, err := auth.Login(cmd.Context(), config, mode); err != nil {
			if errors.Is(err, auth.ErrLoginNeedsBrowser) {
				u.PrintFatalCode(err.Error(), nil, u.ExitUsage)
			}
			u.PrintFatalCode("login failed", err, u.ExitAuth)
		}
		u.PrintSuccess("authenticated successfully - token saved")
	},
}

func init() {
	rootCmd.AddCommand(loginCmd)

	loginCmd.Flags().BoolVar(&loginFlags.manual, "manual", false, "Paste the authorization code (headless/SSH)")
	loginCmd.Flags().BoolVar(&loginFlags.setup, "setup", false, "Run the guided credential-setup wizard")
	loginCmd.Flags().StringVar(&loginFlags.projectID, "project-id", "", "(setup) Google Cloud project ID")
	loginCmd.Flags().StringVar(&loginFlags.clientID, "client-id", "", "(setup) OAuth client ID")
	loginCmd.Flags().StringVar(&loginFlags.clientSecret, "client-secret", "", "(setup) OAuth client secret")
	loginCmd.Flags().BoolVar(&loginFlags.overwrite, "overwrite", false, "(setup) replace existing credentials.json without prompting")
	loginCmd.MarkFlagsMutuallyExclusive("manual", "setup")
}
