package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	gdrive "github.com/tanq16/gdrive/internal"
	"github.com/tanq16/gdrive/internal/ui"
)

var infoFlags struct {
	id string
}

var infoCmd = &cobra.Command{
	Use:   "info <path>",
	Short: "Show file or folder metadata",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		f, err := gdrive.ResolveOrID(args[0], infoFlags.id)
		if err != nil {
			ui.PrintFatal("failed to resolve path", err)
		}

		modified := f.ModifiedTime
		if len(modified) > 19 {
			modified = modified[:19]
			modified = strings.Replace(modified, "T", " ", 1)
		}

		fileType := "file"
		if gdrive.IsFolder(f) {
			fileType = "folder"
		} else if gdrive.IsWorkspaceFile(f) {
			fileType = "workspace"
		}

		ui.PrintGeneric(fmt.Sprintf("Name:     %s", f.Name))
		ui.PrintGeneric(fmt.Sprintf("ID:       %s", f.Id))
		ui.PrintGeneric(fmt.Sprintf("Type:     %s", fileType))
		ui.PrintGeneric(fmt.Sprintf("MIME:     %s", f.MimeType))
		if !gdrive.IsFolder(f) && !gdrive.IsWorkspaceFile(f) {
			ui.PrintGeneric(fmt.Sprintf("Size:     %s", ui.FormatSize(f.Size)))
			if f.Md5Checksum != "" {
				ui.PrintGeneric(fmt.Sprintf("MD5:      %s", f.Md5Checksum))
			}
		}
		ui.PrintGeneric(fmt.Sprintf("Modified: %s", modified))
		if len(f.Parents) > 0 {
			ui.PrintGeneric(fmt.Sprintf("Parent:   %s", f.Parents[0]))
		}
		ui.PrintGeneric(fmt.Sprintf("Trashed:  %v", f.Trashed))
	},
}

func init() {
	rootCmd.AddCommand(infoCmd)
	infoCmd.Flags().StringVar(&infoFlags.id, "id", "", "Use file ID instead of path")
}
