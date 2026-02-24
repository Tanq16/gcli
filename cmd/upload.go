package cmd

import (
	"os"

	"github.com/spf13/cobra"
	gdrive "github.com/tanq16/gdrive/internal"
	"github.com/tanq16/gdrive/internal/ui"
)

var uploadCmd = &cobra.Command{
	Use:   "upload <local> [remote]",
	Short: "Upload file or folder to Google Drive",
	Args:  cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		localPath := args[0]

		// Determine remote parent
		remotePath := "/"
		if len(args) > 1 {
			remotePath = args[1]
		}

		parent, err := gdrive.ResolvePath(remotePath)
		if err != nil {
			ui.PrintFatal("failed to resolve remote path", err)
		}

		if !gdrive.IsFolder(parent) {
			ui.PrintFatal("remote path must be a folder", nil)
		}

		// Check if local path is a file or directory
		info, err := os.Stat(localPath)
		if err != nil {
			ui.PrintFatal("cannot access "+localPath, err)
		}

		if info.IsDir() {
			if err := gdrive.UploadFolder(localPath, parent.Id); err != nil {
				ui.PrintFatal("folder upload failed", err)
			}
			ui.PrintSuccess("folder uploaded successfully")
		} else {
			uploaded, err := gdrive.UploadFile(localPath, parent.Id)
			if err != nil {
				ui.PrintFatal("upload failed", err)
			}
			ui.PrintSuccess("uploaded " + uploaded.Name + " (" + uploaded.Id + ")")
		}
	},
}

func init() {
	rootCmd.AddCommand(uploadCmd)
}
