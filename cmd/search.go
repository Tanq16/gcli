package cmd

import (
	"strings"

	"github.com/spf13/cobra"
	gdrive "github.com/tanq16/gdrive/internal"
	"github.com/tanq16/gdrive/internal/ui"
)

var searchFlags struct {
	fileType   string
	extensions []string
	createdIn  string
	updatedIn  string
	sizeMin    int64
	sizeMax    int64
	limit      int
	sort       string
}

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search for files in Google Drive",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		opts := gdrive.SearchOptions{
			Query:      args[0],
			Type:       searchFlags.fileType,
			Extensions: searchFlags.extensions,
			CreatedIn:  searchFlags.createdIn,
			UpdatedIn:  searchFlags.updatedIn,
			SizeMin:    searchFlags.sizeMin,
			SizeMax:    searchFlags.sizeMax,
			Limit:      searchFlags.limit,
			Sort:       searchFlags.sort,
		}

		files, err := gdrive.Search(opts)
		if err != nil {
			ui.PrintFatal("search failed", err)
		}

		if len(files) == 0 {
			ui.PrintInfo("no results found")
			return
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
	rootCmd.AddCommand(searchCmd)
	searchCmd.Flags().StringVar(&searchFlags.fileType, "type", "", "Filter by type (file, folder)")
	searchCmd.Flags().StringSliceVar(&searchFlags.extensions, "extensions", nil, "Filter by file extensions")
	searchCmd.Flags().StringVar(&searchFlags.createdIn, "created-in", "", "Filter by creation time range (YYYY-MM-DD..YYYY-MM-DD)")
	searchCmd.Flags().StringVar(&searchFlags.updatedIn, "updated-in", "", "Filter by modification time range (YYYY-MM-DD..YYYY-MM-DD)")
	searchCmd.Flags().Int64Var(&searchFlags.sizeMin, "size-min", 0, "Minimum file size in bytes")
	searchCmd.Flags().Int64Var(&searchFlags.sizeMax, "size-max", 0, "Maximum file size in bytes")
	searchCmd.Flags().IntVar(&searchFlags.limit, "limit", 100, "Maximum number of results")
	searchCmd.Flags().StringVar(&searchFlags.sort, "sort", "", "Sort by (name, modifiedTime, size)")
}
