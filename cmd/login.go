package cmd

import (
	"github.com/spf13/cobra"
	"github.com/tanq16/gcli/internal/auth"
	u "github.com/tanq16/gcli/utils"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with Google services",
	Run: func(cmd *cobra.Command, args []string) {
		config, err := auth.LoadCredentials()
		if err != nil {
			u.PrintFatal("failed to load credentials", err)
		}

		token, err := auth.Login(config)
		if err != nil {
			u.PrintFatal("login failed", err)
		}

		_ = token
		u.PrintSuccess("authenticated successfully — token saved")
	},
}

func init() {
	rootCmd.AddCommand(loginCmd)
}
