package driveCmd

import (
	"errors"

	"github.com/spf13/cobra"
	"github.com/tanq16/gcli/internal/auth"
	"github.com/tanq16/gcli/internal/drive"
	u "github.com/tanq16/gcli/utils"
)

var driveFlags struct {
	workers int
	id      bool
	shared  bool
}

var DriveCmd = &cobra.Command{
	Use:   "drive",
	Short: "Google Drive file operations",
	// Runnable so cobra reaches ValidateArgs; a bare parent returns ErrHelp first and a
	// mistyped subcommand would print help and exit 0.
	Args: cobra.NoArgs,
	Run:  func(cmd *cobra.Command, args []string) { _ = cmd.Help() },
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// A parent invocation only prints help, which must work unauthenticated.
		if cmd.HasSubCommands() {
			return
		}
		client, err := auth.GetHTTPClient(cmd.Context())
		if errors.Is(err, auth.ErrNoCredentials) {
			u.PrintFatalCode(err.Error()+"; "+auth.NoCredentialsHint, nil, u.ExitAuth)
		}
		if err != nil {
			u.PrintFatalCode("not authenticated — run 'gcli login'", err, u.ExitAuth)
		}
		if err := drive.Init(client, drive.Options{
			Workers: max(1, driveFlags.workers),
			Shared:  driveFlags.shared,
			ByID:    driveFlags.id,
		}); err != nil {
			u.PrintFatalCode("failed to initialize Drive client", err, u.ExitAuth)
		}
	},
}

func init() {
	DriveCmd.PersistentFlags().IntVarP(&driveFlags.workers, "workers", "w", 4, "Worker-pool size for upload/download/sync")
	DriveCmd.PersistentFlags().BoolVar(&driveFlags.id, "id", false, "Treat remote arguments as Drive IDs (resolved to paths)")
	DriveCmd.PersistentFlags().BoolVarP(&driveFlags.shared, "shared", "s", false, "Operate in the shared namespace (shared drives + shared-with-me)")
}
