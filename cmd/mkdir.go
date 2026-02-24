package cmd

import (
	"github.com/spf13/cobra"
	gdrive "github.com/tanq16/gdrive/internal"
	"github.com/tanq16/gdrive/internal/ui"
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
			folder, err := gdrive.MkdirP(path)
			if err != nil {
				ui.PrintFatal("failed to create directories", err)
			}
			ui.PrintSuccess("created " + folder.Name + " (" + folder.Id + ")")
			return
		}

		parentID, name, err := gdrive.ResolveParent(path)
		if err != nil {
			ui.PrintFatal("failed to resolve parent path", err)
		}

		folder, err := gdrive.CreateFolder(name, parentID)
		if err != nil {
			ui.PrintFatal("failed to create folder", err)
		}

		ui.PrintSuccess("created " + folder.Name + " (" + folder.Id + ")")
	},
}

func init() {
	rootCmd.AddCommand(mkdirCmd)
	mkdirCmd.Flags().BoolVarP(&mkdirFlags.parents, "parents", "p", false, "Create parent directories as needed")
}
