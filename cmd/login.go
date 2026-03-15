package cmd

import (
	"github.com/spf13/cobra"
	"github.com/tanq16/gcli/internal/auth"
	u "github.com/tanq16/gcli/utils"
)

var loginFlags struct {
	deviceLogin bool
	manual      bool
}

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with Google services",
	Run: func(cmd *cobra.Command, args []string) {
		config, err := auth.LoadCredentials()
		if err != nil {
			u.PrintFatal("failed to load credentials", err)
		}

		mode := "default"
		if loginFlags.deviceLogin {
			mode = "device"
		} else if loginFlags.manual {
			mode = "manual"
		}

		if _, err := auth.Login(config, mode); err != nil {
			u.PrintFatal("login failed", err)
		}
		u.PrintSuccess("authenticated successfully — token saved")
	},
}

func init() {
	rootCmd.AddCommand(loginCmd)

	loginCmd.Flags().BoolVar(&loginFlags.deviceLogin, "device-login", false, "Use device code flow (for headless/SSH environments)")
	loginCmd.Flags().BoolVar(&loginFlags.manual, "manual", false, "Manually paste authorization code (last resort)")
	loginCmd.MarkFlagsMutuallyExclusive("device-login", "manual")
}
