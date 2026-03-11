package driveCmd

import (
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/tanq16/gdrive/internal/drive"
	u "github.com/tanq16/gdrive/utils"
)

var downloadFlags struct {
	id string
}

var downloadCmd = &cobra.Command{
	Use:   "download <remote> [local]",
	Short: "Download file or folder from Google Drive",
	Args:  cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		f, err := drive.ResolveOrID(args[0], downloadFlags.id)
		if err != nil {
			u.PrintFatal("failed to resolve remote path", err)
		}

		// Determine local destination
		localPath := f.Name
		if len(args) > 1 {
			localPath = args[1]
		}

		if drive.IsFolder(f) {
			if err := drive.DownloadFolder(f.Id, localPath); err != nil {
				u.PrintFatal("folder download failed", err)
			}
			u.PrintSuccess("folder downloaded to " + localPath)
		} else {
			localPath = filepath.Clean(localPath)
			if err := drive.DownloadFile(f, localPath); err != nil {
				u.PrintFatal("download failed", err)
			}
			u.PrintSuccess("downloaded " + f.Name)
		}
	},
}

func init() {
	DriveCmd.AddCommand(downloadCmd)
	downloadCmd.Flags().StringVar(&downloadFlags.id, "id", "", "Use file ID instead of path")
}
