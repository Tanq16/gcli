package cmd

import (
	"github.com/spf13/cobra"
	gdrive "github.com/tanq16/gdrive/internal"
	"github.com/tanq16/gdrive/internal/ui"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with Google Drive",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// No-op: skip auth init for login command
	},
	Run: func(cmd *cobra.Command, args []string) {
		config, err := gdrive.LoadCredentials()
		if err != nil {
			ui.PrintFatal("failed to load credentials", err)
		}

		token, err := gdrive.Login(config)
		if err != nil {
			ui.PrintFatal("login failed", err)
		}

		_ = token
		ui.PrintSuccess("authenticated successfully — token saved")
	},
}

func init() {
	rootCmd.AddCommand(loginCmd)
}
