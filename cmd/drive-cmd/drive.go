package driveCmd

import (
	"github.com/spf13/cobra"
	syncCmd "github.com/tanq16/gcli/cmd/drive-cmd/sync-cmd"
	"github.com/tanq16/gcli/internal/auth"
	"github.com/tanq16/gcli/internal/drive"
)

var sharedFlag bool

func init() {
	DriveCmd.AddCommand(syncCmd.SyncCmd)
	DriveCmd.PersistentFlags().BoolVarP(&sharedFlag, "shared", "S", false, "Resolve paths from 'Shared with me' instead of My Drive")
}

// DriveCmd is the parent command for all Google Drive operations
var DriveCmd = &cobra.Command{
	Use:   "drive",
	Short: "Google Drive file operations",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		client, err := auth.GetHTTPClient()
		if err != nil {
			return err
		}
		if err := drive.Init(client); err != nil {
			return err
		}
		drive.SharedMode = sharedFlag
		return nil
	},
}
