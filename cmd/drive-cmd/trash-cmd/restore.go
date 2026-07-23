package trashCmd

import (
	"github.com/spf13/cobra"
	"github.com/tanq16/gcli/internal/drive"
	u "github.com/tanq16/gcli/utils"
)

var restoreFlags struct {
	id string
}

var RestoreCmd = &cobra.Command{
	Use:   "restore <path>",
	Short: "Restore a trashed file or folder",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		path := ""
		if len(args) > 0 {
			path = args[0]
		}
		if path == "" && restoreFlags.id == "" {
			u.PrintFatal("provide a path or --id", nil)
		}

		f, err := drive.ResolveOrID(path, restoreFlags.id)
		if err != nil {
			u.PrintFatal("failed to resolve path", err)
		}

		if _, err := drive.RestoreFile(f.Id); err != nil {
			u.PrintFatal("failed to restore "+f.Name, err)
		}

		u.PrintSuccess("restored " + f.Name)
	},
}

func init() {
	RestoreCmd.Flags().StringVarP(&restoreFlags.id, "id", "i", "", "Use file ID instead of path")
}
