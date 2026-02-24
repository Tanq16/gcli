package cmd

import (
	"strings"

	"github.com/spf13/cobra"
	gdrive "github.com/tanq16/gdrive/internal"
	"github.com/tanq16/gdrive/internal/ui"
	drive "google.golang.org/api/drive/v3"
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

		folder, err := gdrive.ResolveOrID(path, listFlags.id)
		if err != nil {
			ui.PrintFatal("failed to resolve path", err)
		}

		if !gdrive.IsFolder(folder) {
			ui.PrintFatal("not a folder: "+folder.Name, nil)
		}

		files, err := gdrive.ListFolder(folder.Id)
		if err != nil {
			ui.PrintFatal("failed to list folder", err)
		}

		if len(files) == 0 {
			ui.PrintInfo("folder is empty")
			return
		}

		// Apply filter if set
		if listFlags.filter != "" {
			filter := strings.ToLower(listFlags.filter)
			var filtered []*drive.File
			for _, f := range files {
				if strings.Contains(strings.ToLower(f.Name), filter) {
					filtered = append(filtered, f)
				}
			}
			files = filtered
			if len(files) == 0 {
				ui.PrintInfo("no items match filter")
				return
			}
		}

		headers := []string{"TYPE", "NAME", "SIZE", "MODIFIED", "ID"}
		var rows [][]string
		for _, f := range files {
			fileType := "file"
			if gdrive.IsFolder(f) {
				fileType = "dir"
			} else if gdrive.IsWorkspaceFile(f) {
				fileType = "gdoc"
			}

			size := ui.FormatSize(f.Size)
			if gdrive.IsFolder(f) || gdrive.IsWorkspaceFile(f) {
				size = "-"
			}

			modified := ""
			if f.ModifiedTime != "" {
				modified = f.ModifiedTime[:16]
				modified = strings.Replace(modified, "T", " ", 1)
			}

			rows = append(rows, []string{fileType, f.Name, size, modified, f.Id})
		}

		ui.PrintTable(headers, rows)
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
	listCmd.Flags().StringVar(&listFlags.id, "id", "", "Use folder ID instead of path")
	listCmd.Flags().StringVarP(&listFlags.filter, "filter", "F", "", "Filter results by name")
}
