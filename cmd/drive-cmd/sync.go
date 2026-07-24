package driveCmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tanq16/gcli/internal/drive"
	u "github.com/tanq16/gcli/utils"
)

var syncFlags struct {
	reverse bool
	backup  bool
	dryRun  bool
	yes     bool
	ignore  []string
}

var syncCmd = &cobra.Command{
	Use:   "sync <local> <remote>",
	Short: "Mirror a local path to Drive (or the reverse) with recoverable deletes",
	Long: "Destructive mirror between a local path and Drive. Arguments are always " +
		"<local> <remote>; --reverse flips data flow (Drive → local). Deletes are " +
		"recoverable: remote deletes go to Drive trash, reverse deletes to .trash.gcli/.",
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		params := drive.SyncParams{
			Local:   args[0],
			Remote:  args[1],
			Reverse: syncFlags.reverse,
			Backup:  syncFlags.backup,
			DryRun:  syncFlags.dryRun,
			Yes:     syncFlags.yes,
			Ignore:  syncFlags.ignore,
			Confirm: confirmDeletes,
		}
		res, err := drive.C().Sync(cmd.Context(), params)
		// Check cancellation first: it can surface as an error from a mid-build tree
		// walk, but must still be a warning + exit 130, never a fatal.
		if cmd.Context().Err() != nil {
			u.PrintWarn("cancelled — partial state remains", nil)
			os.Exit(u.ExitCancelled)
		}
		if err != nil {
			u.PrintFatal("sync failed", err)
		}
		if res.DryRun {
			reportDryRun(res, syncFlags.reverse)
			return
		}
		if res.Aborted {
			u.PrintFatalCode("sync aborted — deletes not confirmed", nil, u.ExitGeneric)
		}
		reportSync(res, syncFlags.reverse)
		if len(res.Errors) > 0 {
			os.Exit(u.ExitPartial)
		}
	},
}

func confirmDeletes(deletes []drive.Item) (bool, error) {
	u.PrintWarn(fmt.Sprintf("%d item(s) on the destination are not on the source and will be deleted:", len(deletes)), nil)
	for _, d := range deletes {
		u.PrintInfo("  delete " + d.RelPath)
	}
	if u.GlobalForAIFlag {
		u.PrintFatalCode("refusing to delete without --yes in --for-ai mode", nil, u.ExitUsage)
	}
	ans, err := u.PromptInput(fmt.Sprintf("Delete %d item(s)? Type 'yes' to confirm:", len(deletes)), "yes/no")
	if err != nil {
		return false, err
	}
	return strings.EqualFold(strings.TrimSpace(ans), "yes"), nil
}

func planCounts(p *drive.Plan) (creates, updates, touches int) {
	for _, it := range p.Files {
		switch it.Op {
		case drive.OpCreate:
			creates++
		case drive.OpUpdate:
			updates++
		case drive.OpTouch:
			touches++
		}
	}
	return
}

func reportDryRun(res *drive.SyncResult, reverse bool) {
	verb := "create"
	if reverse {
		verb = "download"
	}
	creates, updates, touches := planCounts(res.Plan)
	u.PrintInfo(fmt.Sprintf("dry run: %d %s, %d update, %d touch, %d delete, %d unchanged",
		creates, verb, updates, touches, len(res.Plan.Deletes), res.Unchanged))
	for _, d := range res.Plan.MkDirs {
		u.PrintInfo("  mkdir " + d)
	}
	for _, it := range res.Plan.Files {
		u.PrintInfo("  " + opLabel(it.Op) + " " + it.RelPath)
	}
	for _, it := range res.Plan.Deletes {
		u.PrintInfo("  delete " + it.RelPath)
	}
	reportSkipped(res.Skipped)
}

func opLabel(op drive.Op) string {
	switch op {
	case drive.OpCreate:
		return "create"
	case drive.OpUpdate:
		return "update"
	case drive.OpTouch:
		return "touch "
	default:
		return "?"
	}
}

func reportSync(res *drive.SyncResult, reverse bool) {
	moved := "uploaded"
	deleted := "trashed"
	if reverse {
		moved = "downloaded"
		deleted = "removed"
	}
	summary := fmt.Sprintf("%d %s, %d updated, %d touched, %d %s, %d unchanged",
		res.Created, moved, res.Updated, res.Touched, res.Deleted, deleted, res.Unchanged)

	if len(res.Errors) > 0 {
		u.PrintError(fmt.Sprintf("sync completed with %d error(s): %s", len(res.Errors), summary), nil)
		for _, e := range res.Errors {
			u.PrintIndentedError(e.RelPath, e.Err)
		}
	} else {
		u.PrintSuccess("sync complete: " + summary)
	}
	reportSkipped(res.Skipped)
	if res.LocalTrashed > 0 {
		u.PrintInfo(fmt.Sprintf("note: %d local file(s) not on remote were moved to .trash.gcli/", res.LocalTrashed))
	}
}

func reportSkipped(skipped []string) {
	if len(skipped) == 0 {
		return
	}
	u.PrintWarn(fmt.Sprintf("skipped %d unsyncable item(s) (Google-native, shortcuts, symlinks): %s",
		len(skipped), strings.Join(skipped, ", ")), nil)
}

func init() {
	DriveCmd.AddCommand(syncCmd)
	syncCmd.Flags().BoolVarP(&syncFlags.reverse, "reverse", "R", false, "Pull: mirror remote → local (default is push)")
	syncCmd.Flags().BoolVar(&syncFlags.backup, "backup", false, "Rename the existing destination to <name>.bak and mirror into a fresh one")
	syncCmd.Flags().BoolVar(&syncFlags.dryRun, "dry-run", false, "Print the plan and exit without making changes")
	syncCmd.Flags().StringSliceVarP(&syncFlags.ignore, "ignore", "i", nil, "Glob patterns to skip (repeatable/comma), filtering both trees")
	syncCmd.Flags().BoolVarP(&syncFlags.yes, "yes", "y", false, "Skip the delete-confirmation gate")
}
