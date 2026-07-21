package trashCmd

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/tanq16/gcli/internal/drive"
	u "github.com/tanq16/gcli/utils"
)

var emptyFlags struct {
	yes bool
}

var emptyCmd = &cobra.Command{
	Use:   "empty",
	Short: "Permanently delete all items in trash",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if !emptyFlags.yes && !u.GlobalForAIFlag {
			answer, err := u.PromptInput("Permanently delete all trashed items? Type 'yes' to confirm:", "yes/no")
			if err != nil {
				u.PrintFatal("failed to read confirmation", err)
			}
			if strings.ToLower(strings.TrimSpace(answer)) != "yes" {
				u.PrintInfo("aborted")
				return
			}
		}

		if err := drive.EmptyTrash(); err != nil {
			u.PrintFatal("failed to empty trash", err)
		}

		u.PrintSuccess("emptied trash")
	},
}

func init() {
	TrashCmd.AddCommand(emptyCmd)
	emptyCmd.Flags().BoolVarP(&emptyFlags.yes, "yes", "y", false, "Skip confirmation prompt")
}
