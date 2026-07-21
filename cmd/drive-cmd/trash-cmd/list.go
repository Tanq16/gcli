package trashCmd

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/tanq16/gcli/internal/drive"
	u "github.com/tanq16/gcli/utils"
)

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List items in trash",
	Args:    cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		files, err := drive.ListTrashed()
		if err != nil {
			u.PrintFatal("failed to list trash", err)
		}

		if len(files) == 0 {
			u.PrintInfo("trash is empty")
			return
		}

		headers := []string{"TYPE", "NAME", "SIZE", "TRASHED", "ID"}
		var rows [][]string
		for _, f := range files {
			fileType := "file"
			if drive.IsFolder(f) {
				fileType = "dir"
			} else if drive.IsWorkspaceFile(f) {
				fileType = "gdoc"
			}

			size := u.FormatSize(f.Size)
			if drive.IsFolder(f) || drive.IsWorkspaceFile(f) {
				size = "-"
			}

			trashed := ""
			if f.TrashedTime != "" {
				trashed = strings.Replace(f.TrashedTime[:16], "T", " ", 1)
			}

			rows = append(rows, []string{fileType, f.Name, size, trashed, f.Id})
		}

		u.PrintTable(headers, rows)
	},
}

func init() {
	TrashCmd.AddCommand(listCmd)
}
