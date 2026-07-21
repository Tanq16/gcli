package driveCmd

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/tanq16/gcli/internal/drive"
	u "github.com/tanq16/gcli/utils"
)

var purgeFlags struct {
	id  string
	yes bool
}

var purgeCmd = &cobra.Command{
	Use:   "purge <path>",
	Short: "Permanently delete a file or folder, bypassing trash",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		f, err := drive.ResolveOrID(args[0], purgeFlags.id)
		if err != nil {
			u.PrintFatal("failed to resolve path", err)
		}

		if !purgeFlags.yes {
			if u.GlobalForAIFlag {
				u.PrintFatal("refusing to purge without --yes in --for-ai mode", nil)
			}
			answer, err := u.PromptInput("Permanently delete "+f.Name+"? Type 'yes' to confirm:", "yes/no")
			if err != nil {
				u.PrintFatal("failed to read confirmation", err)
			}
			if strings.ToLower(strings.TrimSpace(answer)) != "yes" {
				u.PrintInfo("aborted")
				return
			}
		}

		if err := drive.PurgeFile(f.Id); err != nil {
			u.PrintFatal("failed to purge "+f.Name, err)
		}

		u.PrintSuccess("permanently deleted " + f.Name)
	},
}

func init() {
	DriveCmd.AddCommand(purgeCmd)
	purgeCmd.Flags().StringVarP(&purgeFlags.id, "id", "i", "", "Use file ID instead of path")
	purgeCmd.Flags().BoolVarP(&purgeFlags.yes, "yes", "y", false, "Skip confirmation prompt")
}
