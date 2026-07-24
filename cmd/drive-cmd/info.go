package driveCmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tanq16/gcli/internal/drive"
	u "github.com/tanq16/gcli/utils"
	driveapi "google.golang.org/api/drive/v3"
)

var infoFlags struct {
	revisions bool
}

var infoCmd = &cobra.Command{
	Use:   "info <path>",
	Short: "Show file or folder metadata",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := drive.C()
		f, err := c.ResolveArg(ctx, args[0])
		if err != nil {
			u.PrintFatal("failed to resolve path", err)
		}

		path := "-"
		if p, err := c.ResolveIDToPath(ctx, f.Id); err == nil {
			path = "/" + p
		}
		u.PrintTable(infoRows(f, path))

		if infoFlags.revisions {
			printRevisions(ctx, c, f)
		}
	},
}

func infoRows(f *driveapi.File, path string) ([]string, [][]string) {
	rows := [][]string{
		{"Name", f.Name},
		{"Type", drive.FileType(f)},
		{"Path", path},
	}
	if !drive.IsFolder(f) && !drive.IsWorkspaceFile(f) {
		rows = append(rows, []string{"Size", u.FormatSize(f.Size)})
		if f.Md5Checksum != "" {
			rows = append(rows, []string{"MD5", f.Md5Checksum})
		}
	}
	rows = append(rows,
		[]string{"Created", drive.FormatDriveTime(f.CreatedTime)},
		[]string{"Modified", drive.FormatDriveTime(f.ModifiedTime)},
		[]string{"Owner", ownerOf(f)},
		[]string{"Shared", fmt.Sprintf("%v", f.Shared)},
	)
	if f.WebViewLink != "" {
		rows = append(rows, []string{"WebViewLink", f.WebViewLink})
	}
	rows = append(rows,
		[]string{"ID", f.Id},
		[]string{"Trashed", fmt.Sprintf("%v", f.Trashed)},
	)
	return []string{"FIELD", "VALUE"}, rows
}

func ownerOf(f *driveapi.File) string {
	if len(f.Owners) == 0 {
		return "-"
	}
	o := f.Owners[0]
	if o.EmailAddress != "" {
		return o.EmailAddress
	}
	if o.DisplayName != "" {
		return o.DisplayName
	}
	return "-"
}

func printRevisions(ctx context.Context, c *drive.Client, f *driveapi.File) {
	if drive.IsFolder(f) {
		u.PrintWarn("folders have no revisions — ignoring --revisions", nil)
		return
	}
	revs, err := c.ListRevisions(ctx, f.Id)
	if err != nil {
		u.PrintFatal("failed to list revisions", err)
	}
	if len(revs) == 0 {
		u.PrintInfo("no revisions")
		return
	}
	rows := make([][]string, 0, len(revs))
	for _, r := range revs {
		size := "-"
		if r.Size > 0 {
			size = u.FormatSize(r.Size)
		}
		rows = append(rows, []string{r.Id, drive.FormatDriveTime(r.ModifiedTime), size, fmt.Sprintf("%v", r.KeepForever)})
	}
	u.PrintTable([]string{"REVISION", "MODIFIED", "SIZE", "KEPT"}, rows)
}

func init() {
	DriveCmd.AddCommand(infoCmd)
	infoCmd.Flags().BoolVar(&infoFlags.revisions, "revisions", false, "Also list the file's revision history")
}
