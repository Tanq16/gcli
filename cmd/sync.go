package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	gdrive "github.com/tanq16/gdrive/internal"
	"github.com/tanq16/gdrive/internal/ui"
)

var syncPushFlags struct {
	concurrency int
	ignore      string
}

var syncPullFlags struct {
	concurrency int
	ignore      string
}

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync files between local and Google Drive",
}

var syncPushCmd = &cobra.Command{
	Use:   "push <local> <remote>",
	Short: "Push local directory to Google Drive",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		localPath := args[0]
		remotePath := args[1]

		ignoreList := parseIgnore(syncPushFlags.ignore)

		folder, err := gdrive.ResolvePath(remotePath)
		if err != nil {
			ui.PrintFatal("failed to resolve remote path", err)
		}
		if !gdrive.IsFolder(folder) {
			ui.PrintFatal("remote path must be a folder", nil)
		}

		ui.PrintInfo("building local tree...")
		localTree, err := gdrive.BuildLocalTree(ctx, localPath, ignoreList)
		if err != nil {
			ui.PrintFatal("failed to build local tree", err)
		}

		ui.PrintInfo("building remote tree...")
		remoteTree, err := gdrive.BuildRemoteTree(ctx, folder.Id, "", ignoreList)
		if err != nil {
			ui.PrintFatal("failed to build remote tree", err)
		}

		plan := gdrive.CompareTrees(localTree, remoteTree)

		total := len(plan.Creates) + len(plan.Updates) + len(plan.Deletes)
		if total == 0 {
			ui.PrintSuccess("already in sync")
			return
		}

		ui.PrintInfo(fmt.Sprintf("sync plan: %d creates, %d updates, %d deletes",
			len(plan.Creates), len(plan.Updates), len(plan.Deletes)))

		if err := gdrive.ExecutePush(ctx, plan, localPath, folder.Id, syncPushFlags.concurrency); err != nil {
			ui.PrintFatal("sync push failed", err)
		}

		ui.PrintSuccess("sync push complete")
	},
}

var syncPullCmd = &cobra.Command{
	Use:   "pull <remote> <local>",
	Short: "Pull Google Drive folder to local directory",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		remotePath := args[0]
		localPath := args[1]

		ignoreList := parseIgnore(syncPullFlags.ignore)

		folder, err := gdrive.ResolvePath(remotePath)
		if err != nil {
			ui.PrintFatal("failed to resolve remote path", err)
		}
		if !gdrive.IsFolder(folder) {
			ui.PrintFatal("remote path must be a folder", nil)
		}

		ui.PrintInfo("building remote tree...")
		remoteTree, err := gdrive.BuildRemoteTree(ctx, folder.Id, "", ignoreList)
		if err != nil {
			ui.PrintFatal("failed to build remote tree", err)
		}

		ui.PrintInfo("building local tree...")
		localTree, err := gdrive.BuildLocalTree(context.Background(), localPath, ignoreList)
		if err != nil {
			localTree = &gdrive.FileTree{
				Files: make(map[string]gdrive.FileInfo),
				Dirs:  make(map[string]*gdrive.FileTree),
			}
		}

		plan := gdrive.CompareTrees(remoteTree, localTree)

		total := len(plan.Creates) + len(plan.Updates) + len(plan.Deletes)
		if total == 0 {
			ui.PrintSuccess("already in sync")
			return
		}

		ui.PrintInfo(fmt.Sprintf("sync plan: %d creates, %d updates, %d deletes",
			len(plan.Creates), len(plan.Updates), len(plan.Deletes)))

		if err := gdrive.ExecutePull(ctx, plan, folder.Id, localPath, syncPullFlags.concurrency); err != nil {
			ui.PrintFatal("sync pull failed", err)
		}

		ui.PrintSuccess("sync pull complete")
	},
}

func init() {
	rootCmd.AddCommand(syncCmd)
	syncCmd.AddCommand(syncPushCmd)
	syncCmd.AddCommand(syncPullCmd)

	syncPushCmd.Flags().IntVarP(&syncPushFlags.concurrency, "concurrency", "t", 4, "Number of concurrent operations")
	syncPushCmd.Flags().StringVarP(&syncPushFlags.ignore, "ignore", "i", "", "Comma-separated names to skip")

	syncPullCmd.Flags().IntVarP(&syncPullFlags.concurrency, "concurrency", "t", 4, "Number of concurrent operations")
	syncPullCmd.Flags().StringVarP(&syncPullFlags.ignore, "ignore", "i", "", "Comma-separated names to skip")
}

func parseIgnore(ignore string) []string {
	if ignore == "" {
		return nil
	}
	parts := strings.Split(ignore, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
