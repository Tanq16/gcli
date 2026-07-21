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
	Use:     "delete <path>",
	Aliases: []string{"rm"},
	Short:   "Move a file or folder to trash",
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		f, err := drive.ResolveOrID(args[0], deleteFlags.id)
		if err != nil {
			u.PrintFatal("failed to resolve path", err)
		}

		if _, err := drive.TrashFile(f.Id); err != nil {
			u.PrintFatal("failed to trash "+f.Name, err)
		}

		u.PrintSuccess("moved " + f.Name + " to trash")
	},
}

func init() {
	DriveCmd.AddCommand(deleteCmd)
	deleteCmd.Flags().StringVarP(&deleteFlags.id, "id", "i", "", "Use file ID instead of path")
}
