package cmd

import (
	"github.com/spf13/cobra"
	"github.com/tanq16/gcli/internal/auth"
	u "github.com/tanq16/gcli/utils"
)

var logoutFlags struct {
	localOnly bool
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Revoke and remove the stored credentials",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if err := auth.Logout(cmd.Context(), logoutFlags.localOnly); err != nil {
			u.PrintFatal("logout failed", err)
		}
		u.PrintSuccess("logged out - token removed")
	},
}

func init() {
	rootCmd.AddCommand(logoutCmd)

	logoutCmd.Flags().BoolVar(&logoutFlags.localOnly, "local-only", false, "Delete the local token without revoking it with Google")
}
