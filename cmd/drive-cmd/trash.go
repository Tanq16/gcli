package driveCmd

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/tanq16/gcli/internal/drive"
	u "github.com/tanq16/gcli/utils"
	driveapi "google.golang.org/api/drive/v3"
)

var trashListFlags struct {
	withID bool
}

var trashEmptyFlags struct {
	yes bool
}

var trashCmd = &cobra.Command{
	Use:   "trash",
	Short: "Manage trashed items",
}

var trashListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List items in trash",
	Args:    cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		files, err := drive.C().ListTrashed(cmd.Context())
		if err != nil {
			u.PrintFatal("failed to list trash", err)
		}
		if len(files) == 0 {
			u.PrintInfo("trash is empty")
			return
		}
		u.PrintTable(trashColumns(files, trashListFlags.withID))
	},
}

var trashRestoreCmd = &cobra.Command{
	Use:   "restore <name>",
	Short: "Restore a trashed file or folder",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		c := drive.C()

		var (
			f   *driveapi.File
			err error
		)
		if c.ByID() {
			f, err = c.GetFile(ctx, args[0])
		} else {
			f, err = c.FindTrashed(ctx, args[0])
		}
		if err != nil {
			u.PrintFatal("failed to find trashed item", err)
		}
		if _, err := c.RestoreFile(ctx, f.Id); err != nil {
			u.PrintFatal("failed to restore "+f.Name, err)
		}
		u.PrintSuccess("restored " + f.Name)
	},
}

var trashEmptyCmd = &cobra.Command{
	Use:   "empty",
	Short: "Permanently delete all items in trash",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if !trashEmptyFlags.yes {
			if u.GlobalForAIFlag {
				u.PrintFatalCode("refusing to empty trash without --yes in --for-ai mode", nil, u.ExitUsage)
			}
			answer, err := u.PromptInput("Permanently delete all trashed items? Type 'yes' to confirm:", "yes/no")
			if err != nil {
				u.PrintFatal("failed to read confirmation", err)
			}
			if strings.ToLower(strings.TrimSpace(answer)) != "yes" {
				u.PrintInfo("aborted")
				return
			}
		}
		if err := drive.C().EmptyTrash(cmd.Context()); err != nil {
			u.PrintFatal("failed to empty trash", err)
		}
		u.PrintSuccess("emptied trash")
	},
}

func trashColumns(files []*driveapi.File, withID bool) ([]string, [][]string) {
	headers := []string{"TYPE", "NAME", "SIZE", "TRASHED"}
	if withID {
		headers = append(headers, "ID")
	}
	rows := make([][]string, 0, len(files))
	for _, f := range files {
		row := []string{drive.FileType(f), f.Name, fileSize(f), drive.FormatDriveTime(f.TrashedTime)}
		if withID {
			row = append(row, f.Id)
		}
		rows = append(rows, row)
	}
	return headers, rows
}

func init() {
	DriveCmd.AddCommand(trashCmd)
	trashCmd.AddCommand(trashListCmd)
	trashCmd.AddCommand(trashRestoreCmd)
	trashCmd.AddCommand(trashEmptyCmd)
	trashListCmd.Flags().BoolVar(&trashListFlags.withID, "with-id", false, "Include the ID column")
	trashEmptyCmd.Flags().BoolVarP(&trashEmptyFlags.yes, "yes", "y", false, "Skip confirmation prompt")
}
