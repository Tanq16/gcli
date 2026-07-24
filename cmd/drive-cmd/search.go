package driveCmd

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/tanq16/gcli/internal/drive"
	u "github.com/tanq16/gcli/utils"
	driveapi "google.golang.org/api/drive/v3"
)

var searchFlags struct {
	in       string
	content  bool
	fileType string
	ext      []string
	created  string
	modified string
	sizeMin  string
	sizeMax  string
	limit    int
	sort     string
	withID   bool
}

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search for files in Google Drive",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := drive.C()

		sizeMin, err := drive.ParseHumanSize(searchFlags.sizeMin)
		if err != nil {
			u.PrintFatal("invalid --size-min", err)
		}
		sizeMax, err := drive.ParseHumanSize(searchFlags.sizeMax)
		if err != nil {
			u.PrintFatal("invalid --size-max", err)
		}

		files, err := c.Search(ctx, drive.SearchOptions{
			Query:    args[0],
			In:       searchFlags.in,
			Content:  searchFlags.content,
			Type:     searchFlags.fileType,
			Ext:      searchFlags.ext,
			Created:  searchFlags.created,
			Modified: searchFlags.modified,
			SizeMin:  sizeMin,
			SizeMax:  sizeMax,
			Limit:    searchFlags.limit,
			Sort:     searchFlags.sort,
		})
		if err != nil {
			u.PrintFatal("search failed", err)
		}
		if len(files) == 0 {
			u.PrintInfo("no results found")
			return
		}

		if searchFlags.in == "" {
			u.PrintTable(searchColumns(ctx, c, files, searchFlags.withID))
			return
		}
		u.PrintTable(fileColumns(files, searchFlags.withID))
	},
}

func searchColumns(ctx context.Context, c *drive.Client, files []*driveapi.File, withID bool) ([]string, [][]string) {
	headers := []string{"TYPE", "NAME", "SIZE", "MODIFIED", "PATH"}
	if withID {
		headers = append(headers, "ID")
	}
	parentPaths, allResolved := c.ResolveParentPaths(ctx, files)
	if !allResolved {
		u.PrintWarn("some result paths could not be resolved", nil)
	}
	rows := make([][]string, 0, len(files))
	for _, f := range files {
		path := "-"
		if len(f.Parents) > 0 {
			if p, ok := parentPaths[f.Parents[0]]; ok {
				path = p
			}
		}
		row := []string{drive.FileType(f), f.Name, fileSize(f), drive.FormatDriveTime(f.ModifiedTime), path}
		if withID {
			row = append(row, f.Id)
		}
		rows = append(rows, row)
	}
	return headers, rows
}

func init() {
	DriveCmd.AddCommand(searchCmd)
	searchCmd.Flags().StringVar(&searchFlags.in, "in", "", "Restrict to a folder (path, or ID under --id)")
	searchCmd.Flags().BoolVar(&searchFlags.content, "content", false, "Full-text search instead of name matching")
	searchCmd.Flags().StringVarP(&searchFlags.fileType, "type", "t", "", "Filter by type (folder, file, doc, sheet, slide)")
	searchCmd.Flags().StringSliceVarP(&searchFlags.ext, "ext", "e", nil, "Filter by file extensions (repeatable/comma)")
	searchCmd.Flags().StringVar(&searchFlags.created, "created", "", "Created within a duration (7d) or date range (YYYY-MM-DD..YYYY-MM-DD)")
	searchCmd.Flags().StringVar(&searchFlags.modified, "modified", "", "Modified within a duration or date range")
	searchCmd.Flags().StringVar(&searchFlags.sizeMin, "size-min", "", "Minimum size (e.g. 10MB)")
	searchCmd.Flags().StringVar(&searchFlags.sizeMax, "size-max", "", "Maximum size (e.g. 1GB)")
	searchCmd.Flags().IntVarP(&searchFlags.limit, "limit", "n", 100, "Maximum number of results")
	searchCmd.Flags().StringVar(&searchFlags.sort, "sort", "modified", "Sort by (name, modified, size)")
	searchCmd.Flags().BoolVar(&searchFlags.withID, "with-id", false, "Include the ID column")
}
