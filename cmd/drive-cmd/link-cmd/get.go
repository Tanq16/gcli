package linkCmd

import (
	"github.com/spf13/cobra"
	"github.com/tanq16/gcli/internal/drive"
	u "github.com/tanq16/gcli/utils"
)

var getFlags struct {
	id string
}

var getCmd = &cobra.Command{
	Use:   "get <path>",
	Short: "Show the public share link, if any",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		path := ""
		if len(args) > 0 {
			path = args[0]
		}
		if path == "" && getFlags.id == "" {
			u.PrintFatal("provide a path or --id", nil)
		}

		f, err := drive.ResolveOrID(path, getFlags.id)
		if err != nil {
			u.PrintFatal("failed to resolve path", err)
		}

		perm, link, err := drive.GetShareLink(f.Id)
		if err != nil {
			u.PrintFatal("failed to get share link", err)
		}
		if perm == nil {
			u.PrintInfo(f.Name + " has no public share link")
			return
		}

		u.PrintInfo(f.Name + " is shared (" + perm.Role + ")")
		u.PrintGeneric(link)
	},
}

func init() {
	LinkCmd.AddCommand(getCmd)
	getCmd.Flags().StringVarP(&getFlags.id, "id", "i", "", "Use file ID instead of path")
}
