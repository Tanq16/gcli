package driveCmd

import (
	"github.com/spf13/cobra"
	"github.com/tanq16/gcli/internal/drive"
	u "github.com/tanq16/gcli/utils"
)

var mkdirFlags struct {
	parents bool
}

var mkdirCmd = &cobra.Command{
	Use:   "mkdir <path>",
	Short: "Create a folder in Google Drive",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		path := args[0]

		if mkdirFlags.parents {
			folder, err := drive.MkdirP(path)
			if err != nil {
				u.PrintFatal("failed to create directories", err)
			}
			u.PrintSuccess("created " + folder.Name + " (" + folder.Id + ")")
			return
		}

		parentID, name, err := drive.ResolveParent(path)
		if err != nil {
			u.PrintFatal("failed to resolve parent path", err)
		}

		folder, err := drive.CreateFolder(name, parentID)
		if err != nil {
			u.PrintFatal("failed to create folder", err)
		}

		u.PrintSuccess("created " + folder.Name + " (" + folder.Id + ")")
	},
}

func init() {
	DriveCmd.AddCommand(mkdirCmd)
	mkdirCmd.Flags().BoolVarP(&mkdirFlags.parents, "parents", "p", false, "Create parent directories as needed")
}
