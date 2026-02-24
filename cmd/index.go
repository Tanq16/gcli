package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	gdrive "github.com/tanq16/gdrive/internal"
	"github.com/tanq16/gdrive/internal/ui"
)

var indexSearchFlags struct {
	excludeDirs  []string
	excludeFiles []string
}

var indexCmd = &cobra.Command{
	Use:   "index [path]",
	Short: "Build offline file index",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		rootID := "root"
		if len(args) > 0 {
			folder, err := gdrive.ResolvePath(args[0])
			if err != nil {
				ui.PrintFatal("failed to resolve path", err)
			}
			rootID = folder.Id
		}

		store, err := gdrive.BuildIndex(rootID)
		if err != nil {
			ui.PrintFatal("failed to build index", err)
		}

		if err := gdrive.SaveIndex(store); err != nil {
			ui.PrintFatal("failed to save index", err)
		}

		ui.PrintSuccess(fmt.Sprintf("indexed %d items", len(store.Items)))
	},
}

var indexSearchCmd = &cobra.Command{
	Use:   "search <regex>",
	Short: "Search offline index with regex pattern",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		results, err := gdrive.SearchIndex(args[0], indexSearchFlags.excludeDirs, indexSearchFlags.excludeFiles)
		if err != nil {
			ui.PrintFatal("index search failed", err)
		}

		if len(results) == 0 {
			ui.PrintInfo("no matches found")
			return
		}

		headers := []string{"TYPE", "NAME", "PATH", "SIZE", "ID"}
		var rows [][]string
		for _, item := range results {
			size := ui.FormatSize(item.Size)
			if item.Type == "folder" {
				size = "-"
			}

			rows = append(rows, []string{item.Type, item.Name, item.Path, size, item.ID})
		}

		ui.PrintTable(headers, rows)
	},
}

func init() {
	rootCmd.AddCommand(indexCmd)
	indexCmd.AddCommand(indexSearchCmd)

	indexSearchCmd.Flags().StringSliceVar(&indexSearchFlags.excludeDirs, "exclude-dirs", nil, "Directory patterns to exclude")
	indexSearchCmd.Flags().StringSliceVar(&indexSearchFlags.excludeFiles, "exclude-files", nil, "File patterns to exclude")
}
