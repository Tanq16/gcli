package driveCmd

import (
	"context"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tanq16/gcli/internal/drive"
	u "github.com/tanq16/gcli/utils"
	driveapi "google.golang.org/api/drive/v3"
)

var listFlags struct {
	withID bool
	filter string
}

var listCmd = &cobra.Command{
	Use:     "list [path]",
	Aliases: []string{"ls"},
	Short:   "List folder contents",
	Args:    cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := drive.C()
		path := "/"
		if len(args) > 0 {
			path = args[0]
		}

		files, empty := listEntries(ctx, c, path)
		if empty {
			return
		}

		if listFlags.filter != "" {
			filter := strings.ToLower(listFlags.filter)
			var kept []*driveapi.File
			for _, f := range files {
				if strings.Contains(strings.ToLower(f.Name), filter) {
					kept = append(kept, f)
				}
			}
			files = kept
			if len(files) == 0 {
				u.PrintInfo("no items match filter")
				return
			}
		}

		u.PrintTable(fileColumns(files, listFlags.withID))
	},
}

// listEntries resolves the listing for a path, handling the shared-namespace root
// (union of shared drives + shared-with-me) specially. The bool reports an empty
// listing already communicated to the user.
func listEntries(ctx context.Context, c *drive.Client, path string) ([]*driveapi.File, bool) {
	if c.Shared() && !c.ByID() && strings.Trim(path, "/") == "" {
		var files []*driveapi.File
		drives, err := c.ListSharedDrives(ctx)
		if err != nil {
			u.PrintFatal("failed to list shared drives", err)
		}
		for _, d := range drives {
			files = append(files, &driveapi.File{Id: d.Id, Name: d.Name, MimeType: "application/vnd.google-apps.folder", DriveId: d.Id})
		}
		swm, err := c.ListSharedWithMe(ctx)
		if err != nil {
			u.PrintFatal("failed to list shared-with-me items", err)
		}
		files = append(files, swm...)
		if len(files) == 0 {
			u.PrintInfo("no shared items found")
			return nil, true
		}
		return files, false
	}

	folder, err := c.ResolveArg(ctx, path)
	if err != nil {
		u.PrintFatal("failed to resolve path", err)
	}
	if !drive.IsFolder(folder) {
		u.PrintFatalCode("not a folder: "+folder.Name, nil, u.ExitUsage)
	}
	files, err := c.ListFolder(ctx, folder)
	if err != nil {
		u.PrintFatal("failed to list folder", err)
	}
	if len(files) == 0 {
		u.PrintInfo("folder is empty")
		return nil, true
	}
	return files, false
}

// fileColumns builds the ls/search table: TYPE, NAME, SIZE, MODIFIED, and ID only
// when withID is set.
func fileColumns(files []*driveapi.File, withID bool) ([]string, [][]string) {
	headers := []string{"TYPE", "NAME", "SIZE", "MODIFIED"}
	if withID {
		headers = append(headers, "ID")
	}
	rows := make([][]string, 0, len(files))
	for _, f := range files {
		row := []string{drive.FileType(f), f.Name, fileSize(f), drive.FormatDriveTime(f.ModifiedTime)}
		if withID {
			row = append(row, f.Id)
		}
		rows = append(rows, row)
	}
	return headers, rows
}

func fileSize(f *driveapi.File) string {
	if drive.IsFolder(f) || drive.IsWorkspaceFile(f) {
		return "-"
	}
	return u.FormatSize(f.Size)
}

func init() {
	DriveCmd.AddCommand(listCmd)
	listCmd.Flags().BoolVar(&listFlags.withID, "with-id", false, "Include the ID column")
	listCmd.Flags().StringVarP(&listFlags.filter, "filter", "f", "", "Filter results by name substring")
}
