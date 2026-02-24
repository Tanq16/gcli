package cmd

import (
	"github.com/spf13/cobra"
	gdrive "github.com/tanq16/gdrive/internal"
	"github.com/tanq16/gdrive/internal/ui"
)

var deleteFlags struct {
	id string
}

var deleteCmd = &cobra.Command{
	Use:   "delete <path>",
	Short: "Delete a file or folder",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		f, err := gdrive.ResolveOrID(args[0], deleteFlags.id)
		if err != nil {
			ui.PrintFatal("failed to resolve path", err)
		}

		if err := gdrive.DeleteFile(f.Id); err != nil {
			ui.PrintFatal("failed to delete "+f.Name, err)
		}

		ui.PrintSuccess("deleted " + f.Name)
	},
}

func init() {
	rootCmd.AddCommand(deleteCmd)
	deleteCmd.Flags().StringVar(&deleteFlags.id, "id", "", "Use file ID instead of path")
}
