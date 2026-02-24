package cmd

import (
	"path"

	"github.com/spf13/cobra"
	gdrive "github.com/tanq16/gdrive/internal"
	"github.com/tanq16/gdrive/internal/ui"
)

var copyFlags struct {
	name string
}

var copyCmd = &cobra.Command{
	Use:   "copy <src> <dst>",
	Short: "Copy a file to a destination folder",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		src, err := gdrive.ResolvePath(args[0])
		if err != nil {
			ui.PrintFatal("failed to resolve source", err)
		}

		dst, err := gdrive.ResolvePath(args[1])
		if err != nil {
			ui.PrintFatal("failed to resolve destination", err)
		}

		if !gdrive.IsFolder(dst) {
			ui.PrintFatal("destination must be a folder", nil)
		}

		name := copyFlags.name
		if name == "" {
			name = path.Base(src.Name)
		}

		copied, err := gdrive.CopyFile(src.Id, name, dst.Id)
		if err != nil {
			ui.PrintFatal("failed to copy "+src.Name, err)
		}

		ui.PrintSuccess("copied to " + copied.Name + " (" + copied.Id + ")")
	},
}

func init() {
	rootCmd.AddCommand(copyCmd)
	copyCmd.Flags().StringVar(&copyFlags.name, "name", "", "Name for the copy")
}
