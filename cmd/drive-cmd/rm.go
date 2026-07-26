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
		trashed := 0
		var errs []error
		for _, arg := range args {
			// Without this an abort reports itself once per remaining argument.
			if ctx.Err() != nil {
				break
			}
			f, err := c.ResolveArg(ctx, arg)
			if err != nil {
				u.PrintError("failed to resolve "+arg, err)
				errs = append(errs, err)
				continue
			}
			if err := c.TrashFile(ctx, f.Id); err != nil {
				u.PrintError("failed to trash "+f.Name, err)
				errs = append(errs, err)
				continue
			}
			c.InvalidatePath(arg)
			trashed++
			u.PrintSuccess("moved " + f.Name + " to trash")
		}
		if code := drive.BatchExitCode(trashed, errs); code != 0 {
			os.Exit(code)
		}
	},
}

func init() {
	DriveCmd.AddCommand(rmCmd)
}
