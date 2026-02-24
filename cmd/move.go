package cmd

import (
	"path"

	"github.com/spf13/cobra"
	gdrive "github.com/tanq16/gdrive/internal"
	"github.com/tanq16/gdrive/internal/ui"
)

var moveCmd = &cobra.Command{
	Use:   "move <src> <dst>",
	Short: "Move or rename a file or folder",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		src, err := gdrive.ResolvePath(args[0])
		if err != nil {
			ui.PrintFatal("failed to resolve source", err)
		}

		currentParentID := ""
		if len(src.Parents) > 0 {
			currentParentID = src.Parents[0]
		}

		// Resolve destination parent and determine new name
		dstParentID, dstName, err := gdrive.ResolveParent(args[1])
		if err != nil {
			// If parent resolution fails, treat dst as just a rename in the same folder
			dstName = path.Base(args[1])
			dstParentID = currentParentID
		}

		moved, err := gdrive.MoveFile(src.Id, dstName, currentParentID, dstParentID)
		if err != nil {
			ui.PrintFatal("failed to move "+src.Name, err)
		}

		ui.PrintSuccess("moved to " + moved.Name + " (" + moved.Id + ")")
	},
}

func init() {
	rootCmd.AddCommand(moveCmd)
}
