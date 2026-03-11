package driveCmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tanq16/gdrive/internal/drive"
	u "github.com/tanq16/gdrive/utils"
)

var infoFlags struct {
	id string
}

var infoCmd = &cobra.Command{
	Use:   "info <path>",
	Short: "Show file or folder metadata",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		f, err := drive.ResolveOrID(args[0], infoFlags.id)
		if err != nil {
			u.PrintFatal("failed to resolve path", err)
		}

		modified := f.ModifiedTime
		if len(modified) > 19 {
			modified = modified[:19]
			modified = strings.Replace(modified, "T", " ", 1)
		}

		fileType := "file"
		if drive.IsFolder(f) {
			fileType = "folder"
		} else if drive.IsWorkspaceFile(f) {
			fileType = "workspace"
		}

		u.PrintGeneric(fmt.Sprintf("Name:     %s", f.Name))
		u.PrintGeneric(fmt.Sprintf("ID:       %s", f.Id))
		u.PrintGeneric(fmt.Sprintf("Type:     %s", fileType))
		u.PrintGeneric(fmt.Sprintf("MIME:     %s", f.MimeType))
		if !drive.IsFolder(f) && !drive.IsWorkspaceFile(f) {
			u.PrintGeneric(fmt.Sprintf("Size:     %s", u.FormatSize(f.Size)))
			if f.Md5Checksum != "" {
				u.PrintGeneric(fmt.Sprintf("MD5:      %s", f.Md5Checksum))
			}
		}
		u.PrintGeneric(fmt.Sprintf("Modified: %s", modified))
		if len(f.Parents) > 0 {
			u.PrintGeneric(fmt.Sprintf("Parent:   %s", f.Parents[0]))
		}
		u.PrintGeneric(fmt.Sprintf("Trashed:  %v", f.Trashed))
	},
}

func init() {
	DriveCmd.AddCommand(infoCmd)
	infoCmd.Flags().StringVar(&infoFlags.id, "id", "", "Use file ID instead of path")
}
