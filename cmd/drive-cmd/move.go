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

// moveDestination resolves the destination into a (parentID, newName) pair. An
// existing folder means move-into (name unchanged); otherwise the destination's
// parent must resolve — an unresolvable parent is a hard error, never a silent
// in-place rename (the removed CL-03 footgun).
func moveDestination(ctx context.Context, c *drive.Client, dst string) (string, string) {
	if f, err := c.ResolveArg(ctx, dst); err == nil {
		if !drive.IsFolder(f) {
			u.PrintFatalCode("destination '"+f.Name+"' exists and is not a folder", nil, u.ExitUsage)
		}
		return f.Id, ""
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
