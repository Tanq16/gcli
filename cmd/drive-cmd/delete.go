package driveCmd

import (
	"github.com/spf13/cobra"
	"github.com/tanq16/gcli/internal/drive"
	u "github.com/tanq16/gcli/utils"
)

var deleteFlags struct {
	id string
}

var deleteCmd = &cobra.Command{
	Use:   "delete <path>",
	Short: "Delete a file or folder",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		f, err := drive.ResolveOrID(args[0], deleteFlags.id)
		if err != nil {
			u.PrintFatal("failed to resolve path", err)
		}

		if err := drive.DeleteFile(f.Id); err != nil {
			u.PrintFatal("failed to delete "+f.Name, err)
		}

		u.PrintSuccess("deleted " + f.Name)
	},
}

func init() {
	DriveCmd.AddCommand(deleteCmd)
	deleteCmd.Flags().StringVarP(&deleteFlags.id, "id", "i", "", "Use file ID instead of path")
}
