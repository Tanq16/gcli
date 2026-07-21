package linkCmd

import (
	"github.com/spf13/cobra"
	"github.com/tanq16/gcli/internal/drive"
	u "github.com/tanq16/gcli/utils"
)

var removeFlags struct {
	id string
}

var removeCmd = &cobra.Command{
	Use:   "remove <path>",
	Short: "Remove the public share link",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		path := ""
		if len(args) > 0 {
			path = args[0]
		}
		if path == "" && removeFlags.id == "" {
			u.PrintFatal("provide a path or --id", nil)
		}

		f, err := drive.ResolveOrID(path, removeFlags.id)
		if err != nil {
			u.PrintFatal("failed to resolve path", err)
		}

		removed, err := drive.RemoveShareLink(f.Id)
		if err != nil {
			u.PrintFatal("failed to remove share link", err)
		}
		if !removed {
			u.PrintInfo(f.Name + " has no public share link")
			return
		}

		u.PrintSuccess("removed share link from " + f.Name)
	},
}

func init() {
	LinkCmd.AddCommand(removeCmd)
	removeCmd.Flags().StringVarP(&removeFlags.id, "id", "i", "", "Use file ID instead of path")
}
