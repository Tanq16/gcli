package driveCmd

import (
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tanq16/gcli/internal/drive"
	u "github.com/tanq16/gcli/utils"
)

var purgeFlags struct {
	yes bool
}

var purgeCmd = &cobra.Command{
	Use:   "purge <path>...",
	Short: "Permanently delete files or folders, bypassing trash",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := drive.C()

		if !purgeFlags.yes {
			if u.GlobalForAIFlag {
				u.PrintFatalCode("refusing to purge without --yes in --for-ai mode", nil, u.ExitUsage)
			}
			answer, err := u.PromptInput("Permanently delete "+strings.Join(args, ", ")+"? Type 'yes' to confirm:", "yes/no")
			if err != nil {
				u.PrintFatal("failed to read confirmation", err)
			}
			if strings.ToLower(strings.TrimSpace(answer)) != "yes" {
				u.PrintInfo("aborted")
				return
			}
		}

		failed := 0
		for _, arg := range args {
			f, err := c.ResolveArg(ctx, arg)
			if err != nil {
				u.PrintError("failed to resolve "+arg, err)
				failed++
				continue
			}
			if err := c.PurgeFile(ctx, f.Id); err != nil {
				u.PrintError("failed to purge "+f.Name, err)
				failed++
				continue
			}
			c.InvalidatePath(arg)
			u.PrintSuccess("permanently deleted " + f.Name)
		}
		if failed > 0 {
			os.Exit(u.ExitPartial)
		}
	},
}

func init() {
	DriveCmd.AddCommand(purgeCmd)
	purgeCmd.Flags().BoolVarP(&purgeFlags.yes, "yes", "y", false, "Skip confirmation prompt")
}
