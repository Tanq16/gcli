package driveCmd

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/tanq16/gcli/internal/drive"
	u "github.com/tanq16/gcli/utils"
)

var uploadCmd = &cobra.Command{
	Use:   "upload <local> [remote]",
	Short: "Upload file or folder to Google Drive",
	Args:  cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		localPath := args[0]

		remotePath := "/"
		if len(args) > 1 {
			remotePath = args[1]
		}

		parent, err := drive.ResolvePath(remotePath)
		if err != nil {
			u.PrintFatal("failed to resolve remote path", err)
		}

		if !drive.IsFolder(parent) {
			u.PrintFatal("remote path must be a folder", nil)
		}

		info, err := os.Stat(localPath)
		if err != nil {
			u.PrintFatal("cannot access "+localPath, err)
		}

		if info.IsDir() {
			if err := drive.UploadFolder(cmd.Context(), localPath, parent.Id); err != nil {
				u.PrintFatal("folder upload failed", err)
			}
			u.PrintSuccess("folder uploaded successfully")
		} else {
			u.PrintRunning("uploading...")
			uploaded, err := drive.UploadFile(localPath, parent.Id)
			if err != nil {
				u.ClearLines(1)
				u.PrintFatal("upload failed", err)
			}
			u.ClearLines(1)
			u.PrintSuccess("uploaded " + uploaded.Name + " (" + uploaded.Id + ")")
		}
	},
}

func init() {
	DriveCmd.AddCommand(uploadCmd)
}
