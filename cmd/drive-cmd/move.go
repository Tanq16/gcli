package driveCmd

import (
	"path"

	"github.com/spf13/cobra"
	"github.com/tanq16/gcli/internal/drive"
	u "github.com/tanq16/gcli/utils"
)

var moveCmd = &cobra.Command{
	Use:   "move <src> <dst>",
	Short: "Move or rename a file or folder",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		src, err := drive.ResolvePath(args[0])
		if err != nil {
			u.PrintFatal("failed to resolve source", err)
		}

		currentParentID := ""
		if len(src.Parents) > 0 {
			currentParentID = src.Parents[0]
		}

		dstParentID, dstName, err := drive.ResolveParent(args[1])
		if err != nil {
			dstName = path.Base(args[1])
			dstParentID = currentParentID
		}

		moved, err := drive.MoveFile(src.Id, dstName, currentParentID, dstParentID)
		if err != nil {
			u.PrintFatal("failed to move "+src.Name, err)
		}

		u.PrintSuccess("moved to " + moved.Name + " (" + moved.Id + ")")
	},
}

func init() {
	DriveCmd.AddCommand(moveCmd)
}
