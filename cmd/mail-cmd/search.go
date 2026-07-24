package mailCmd

import (
	"github.com/spf13/cobra"
	"github.com/tanq16/gcli/internal/mail"
	u "github.com/tanq16/gcli/utils"
)

var searchFlags struct {
	limit int64
}

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search threads using Gmail search syntax",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		threads, err := mail.SearchThreads(cmd.Context(), args[0], searchFlags.limit)
		if err != nil {
			u.PrintFatal("search failed", err)
		}

		if len(threads) == 0 {
			u.PrintInfo("no threads found")
			return
		}

		u.PrintTable([]string{"ID", "FROM", "SUBJECT", "DATE"}, threadRows(threads))
	},
}

func init() {
	MailCmd.AddCommand(searchCmd)
	searchCmd.Flags().Int64VarP(&searchFlags.limit, "limit", "n", 20, "Maximum number of results")
}
