package linkCmd

import (
	"github.com/spf13/cobra"
	"github.com/tanq16/gcli/internal/drive"
	u "github.com/tanq16/gcli/utils"
)

var createFlags struct {
	id   string
	role string
}

var createCmd = &cobra.Command{
	Use:   "create <path>",
	Short: "Create an 'anyone with the link' share link",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if createFlags.role != "reader" && createFlags.role != "writer" {
			u.PrintFatal("--role must be 'reader' or 'writer'", nil)
		}

		path := ""
		if len(args) > 0 {
			path = args[0]
		}
		if path == "" && createFlags.id == "" {
			u.PrintFatal("provide a path or --id", nil)
		}

		f, err := drive.ResolveOrID(path, createFlags.id)
		if err != nil {
			u.PrintFatal("failed to resolve path", err)
		}

		link, err := drive.CreateShareLink(f.Id, createFlags.role)
		if err != nil {
			u.PrintFatal("failed to create share link", err)
		}

		u.PrintSuccess("shared " + f.Name + " (" + createFlags.role + ")")
		u.PrintGeneric(link)
	},
}

func init() {
	LinkCmd.AddCommand(createCmd)
	createCmd.Flags().StringVarP(&createFlags.id, "id", "i", "", "Use file ID instead of path")
	createCmd.Flags().StringVarP(&createFlags.role, "role", "r", "reader", "Share role (reader or writer)")
}
