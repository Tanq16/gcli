package driveCmd

import (
	"path"

	"github.com/spf13/cobra"
	"github.com/tanq16/gcli/internal/drive"
	u "github.com/tanq16/gcli/utils"
)

var copyFlags struct {
	name string
}

var copyCmd = &cobra.Command{
	Use:     "copy <src> <dst>",
	Aliases: []string{"cp"},
	Short:   "Copy a file to a destination folder",
	Args:    cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		src, err := drive.ResolvePath(args[0])
		if err != nil {
			u.PrintFatal("failed to resolve source", err)
		}

		dst, err := drive.ResolvePath(args[1])
		if err != nil {
			u.PrintFatal("failed to resolve destination", err)
		}

		if !drive.IsFolder(dst) {
			u.PrintFatal("destination must be a folder", nil)
		}

		name := copyFlags.name
		if name == "" {
			name = path.Base(src.Name)
		}

		copied, err := drive.CopyFile(src.Id, name, dst.Id)
		if err != nil {
			u.PrintFatal("failed to copy "+src.Name, err)
		}

		u.PrintSuccess("copied to " + copied.Name + " (" + copied.Id + ")")
	},
}

func init() {
	DriveCmd.AddCommand(copyCmd)
	copyCmd.Flags().StringVarP(&copyFlags.name, "name", "n", "", "Name for the copy")
}
