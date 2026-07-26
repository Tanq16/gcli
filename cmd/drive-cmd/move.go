package driveCmd

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/tanq16/gcli/internal/drive"
	u "github.com/tanq16/gcli/utils"
)

var moveCmd = &cobra.Command{
	Use:     "move <src> <dst>",
	Aliases: []string{"mv"},
	Short:   "Move or rename a file or folder",
	Args:    cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := drive.C()

		src, err := c.ResolveArg(ctx, args[0])
		if err != nil {
			u.PrintFatal("failed to resolve source", err)
		}
		curParent := ""
		if len(src.Parents) > 0 {
			curParent = src.Parents[0]
		}

		newParent, newName := moveDestination(ctx, c, args[1])

		moved, err := c.MoveFile(ctx, src.Id, newName, curParent, newParent)
		if err != nil {
			u.PrintFatal("failed to move "+src.Name, err)
		}
		c.InvalidatePath(args[0])
		c.InvalidatePath(args[1])
		u.PrintSuccess("moved to " + moved.Name + " (" + moved.Id + ")")
	},
}

// Only a destination that genuinely does not exist is a rename; any other lookup failure would silently move the file out of its folder.
func moveDestination(ctx context.Context, c *drive.Client, dst string) (string, string) {
	f, err := c.ResolveArg(ctx, dst)
	switch {
	case err == nil:
		if !drive.IsFolder(f) {
			u.PrintFatalCode("destination '"+f.Name+"' exists and is not a folder", nil, u.ExitUsage)
		}
		return f.Id, ""
	case !drive.IsNotFound(err):
		u.PrintFatal("failed to resolve destination", err)
	}
	parentID, name, err := c.ResolveArgParent(ctx, dst)
	if err != nil {
		u.PrintFatal("failed to resolve destination", err)
	}
	return parentID, name
}

func init() {
	DriveCmd.AddCommand(moveCmd)
}
