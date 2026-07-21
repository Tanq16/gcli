package driveCmd

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/tanq16/gcli/internal/drive"
	u "github.com/tanq16/gcli/utils"
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
		opts := drive.SearchOptions{
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

		files, err := drive.Search(opts)
		if err != nil {
			u.PrintFatal("search failed", err)
		}

		if len(files) == 0 {
			u.PrintInfo("no results found")
			return
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
	DriveCmd.AddCommand(searchCmd)
	searchCmd.Flags().StringVarP(&searchFlags.fileType, "type", "t", "", "Filter by type (file, folder)")
	searchCmd.Flags().StringSliceVarP(&searchFlags.extensions, "extensions", "e", nil, "Filter by file extensions")
	searchCmd.Flags().StringVarP(&searchFlags.createdIn, "created-in", "C", "", "Filter by creation time range (YYYY-MM-DD..YYYY-MM-DD)")
	searchCmd.Flags().StringVarP(&searchFlags.updatedIn, "updated-in", "U", "", "Filter by modification time range (YYYY-MM-DD..YYYY-MM-DD)")
	searchCmd.Flags().Int64VarP(&searchFlags.sizeMin, "size-min", "m", 0, "Minimum file size in bytes")
	searchCmd.Flags().Int64VarP(&searchFlags.sizeMax, "size-max", "M", 0, "Maximum file size in bytes")
	searchCmd.Flags().IntVarP(&searchFlags.limit, "limit", "n", 100, "Maximum number of results")
	searchCmd.Flags().StringVarP(&searchFlags.sort, "sort", "s", "", "Sort by (name, modifiedTime, size)")
}
