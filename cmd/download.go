package cmd

import (
	"path/filepath"

	"github.com/spf13/cobra"
	gdrive "github.com/tanq16/gdrive/internal"
	"github.com/tanq16/gdrive/internal/ui"
)

var downloadFlags struct {
	id string
}

var downloadCmd = &cobra.Command{
	Use:   "download <remote> [local]",
	Short: "Download file or folder from Google Drive",
	Args:  cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		f, err := gdrive.ResolveOrID(args[0], downloadFlags.id)
		if err != nil {
			ui.PrintFatal("failed to resolve remote path", err)
		}

		// Determine local destination
		localPath := f.Name
		if len(args) > 1 {
			localPath = args[1]
		}

		if gdrive.IsFolder(f) {
			if err := gdrive.DownloadFolder(f.Id, localPath); err != nil {
				ui.PrintFatal("folder download failed", err)
			}
			ui.PrintSuccess("folder downloaded to " + localPath)
		} else {
			localPath = filepath.Clean(localPath)
			if err := gdrive.DownloadFile(f, localPath); err != nil {
				ui.PrintFatal("download failed", err)
			}
			ui.PrintSuccess("downloaded " + f.Name)
		}
	},
}

func init() {
	rootCmd.AddCommand(downloadCmd)
	downloadCmd.Flags().StringVar(&downloadFlags.id, "id", "", "Use file ID instead of path")
}
