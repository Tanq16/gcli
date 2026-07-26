package driveCmd

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/tanq16/gcli/internal/drive"
	u "github.com/tanq16/gcli/utils"
)

var catFlags struct {
	format string
}

var catCmd = &cobra.Command{
	Use:   "cat <remote>",
	Short: "Stream a remote file's contents to stdout",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := drive.C().Cat(cmd.Context(), args[0], catFlags.format, os.Stdout); err != nil {
			u.PrintFatal("cat failed", err)
		}
	},
}

func init() {
	DriveCmd.AddCommand(catCmd)
	catCmd.Flags().StringVar(&catFlags.format, "format", "", "Export format for Workspace files (pdf, md, txt, csv, ...)")
}
