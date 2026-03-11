package driveCmd

import (
	"github.com/spf13/cobra"
	"github.com/tanq16/gdrive/internal/auth"
	"github.com/tanq16/gdrive/internal/drive"
)

// DriveCmd is the parent command for all Google Drive operations
var DriveCmd = &cobra.Command{
	Use:   "drive",
	Short: "Google Drive file operations",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		client, err := auth.GetHTTPClient()
		if err != nil {
			return err
		}
		debug, _ := cmd.Root().PersistentFlags().GetBool("debug")
		return drive.Init(client, debug)
	},
}
