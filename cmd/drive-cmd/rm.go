package driveCmd

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/tanq16/gcli/internal/drive"
	u "github.com/tanq16/gcli/utils"
)

var rmCmd = &cobra.Command{
	Use:     "rm <path>...",
	Aliases: []string{"remove", "delete"},
	Short:   "Move files or folders to trash (recoverable)",
	Args:    cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := drive.C()
		failed := 0
		for _, arg := range args {
			f, err := c.ResolveArg(ctx, arg)
			if err != nil {
				u.PrintError("failed to resolve "+arg, err)
				failed++
				continue
			}
			if err := c.TrashFile(ctx, f.Id); err != nil {
				u.PrintError("failed to trash "+f.Name, err)
				failed++
				continue
			}
			c.InvalidatePath(arg)
			u.PrintSuccess("moved " + f.Name + " to trash")
		}
		if failed > 0 {
			os.Exit(u.ExitPartial)
		}
	},
}

func init() {
	DriveCmd.AddCommand(rmCmd)
}
