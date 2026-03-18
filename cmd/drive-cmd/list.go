package driveCmd

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/tanq16/gcli/internal/drive"
	u "github.com/tanq16/gcli/utils"
	driveapi "google.golang.org/api/drive/v3"
)

var listFlags struct {
	id     string
	filter string
}

var listCmd = &cobra.Command{
	Use:   "list [path]",
	Short: "List folder contents",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		path := "/"
		if len(args) > 0 {
			path = args[0]
		}

		folder, err := drive.ResolveOrID(path, listFlags.id)
		if err != nil {
			u.PrintFatal("failed to resolve path", err)
		}

		if !drive.IsFolder(folder) {
			u.PrintFatal("not a folder: "+folder.Name, nil)
		}

		files, err := drive.ListFolder(folder.Id)
		if err != nil {
			u.PrintFatal("failed to list folder", err)
		}

		if len(files) == 0 {
			u.PrintInfo("folder is empty")
			return
		}

		if listFlags.filter != "" {
			filter := strings.ToLower(listFlags.filter)
			var filtered []*driveapi.File
			for _, f := range files {
				if strings.Contains(strings.ToLower(f.Name), filter) {
					filtered = append(filtered, f)
				}
			}
			files = filtered
			if len(files) == 0 {
				u.PrintInfo("no items match filter")
				return
			}
		}

		headers := []string{"TYPE", "NAME", "SIZE", "MODIFIED", "ID"}
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

			modified := ""
			if f.ModifiedTime != "" {
				modified = f.ModifiedTime[:16]
				modified = strings.Replace(modified, "T", " ", 1)
			}

			rows = append(rows, []string{fileType, f.Name, size, modified, f.Id})
		}

		u.PrintTable(headers, rows)
	},
}

func init() {
	DriveCmd.AddCommand(listCmd)
	listCmd.Flags().StringVarP(&listFlags.id, "id", "i", "", "Use folder ID instead of path")
	listCmd.Flags().StringVarP(&listFlags.filter, "filter", "F", "", "Filter results by name")
}
