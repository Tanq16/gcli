package driveCmd

import (
	"github.com/spf13/cobra"
	"github.com/tanq16/gcli/internal/drive"
	u "github.com/tanq16/gcli/utils"
)

var downloadFlags struct {
	format   string
	revision string
}

var downloadCmd = &cobra.Command{
	Use:     "download <remote> [local]",
	Aliases: []string{"dl"},
	Short:   "Download a file or folder from Google Drive",
	Args:    cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := drive.C()
		local := "."
		if len(args) > 1 {
			local = args[1]
		}

		if downloadFlags.revision != "" {
			res, err := c.DownloadRev(ctx, args[0], local, downloadFlags.revision)
			if err != nil {
				u.PrintFatal("download failed", err)
			}
			reportTransfer(ctx, "downloaded", res)
			return
		}

		res, err := c.Download(ctx, args[0], local, downloadFlags.format)
		if err != nil {
			u.PrintFatal("download failed", err)
		}
		reportTransfer(ctx, "downloaded", res)
	},
}

func init() {
	DriveCmd.AddCommand(downloadCmd)
	downloadCmd.Flags().StringVar(&downloadFlags.format, "format", "", "Export format for Workspace files (pdf, docx, md, csv, xlsx, ...)")
	downloadCmd.Flags().StringVar(&downloadFlags.revision, "revision", "", "Download a specific historical revision by ID (single file)")
}
