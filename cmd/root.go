package cmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	driveCmd "github.com/tanq16/gcli/cmd/drive-cmd"
	mailCmd "github.com/tanq16/gcli/cmd/mail-cmd"
	u "github.com/tanq16/gcli/utils"
)

var AppVersion = "dev-build"
var debugFlag bool
var forAIFlag bool

var rootCmd = &cobra.Command{
	Use:           "gcli",
	Short:         "CLI tool for Google Drive and Gmail",
	Version:       AppVersion,
	SilenceUsage:  true,
	SilenceErrors: true,
	CompletionOptions: cobra.CompletionOptions{
		HiddenDefaultCmd: true,
	},
}

func Execute() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	// Only cobra arg/flag validation surfaces an error here; every other failure
	// exits through the printer with its own code.
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		os.Exit(u.ExitUsage)
	}
}

func setupLogs() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	output := zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: time.DateTime,
		NoColor:    false,
	}
	log.Logger = zerolog.New(output).With().Timestamp().Logger()
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if debugFlag {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
		u.GlobalDebugFlag = true
	}
	if forAIFlag {
		zerolog.SetGlobalLevel(zerolog.Disabled)
		u.GlobalForAIFlag = true
	}
}

func init() {
	rootCmd.SetHelpCommand(&cobra.Command{Hidden: true})
	rootCmd.PersistentFlags().BoolVar(&debugFlag, "debug", false, "Enable debug logging")
	rootCmd.PersistentFlags().BoolVar(&forAIFlag, "for-ai", false, "AI-friendly output (plain text, piped input)")
	rootCmd.MarkFlagsMutuallyExclusive("debug", "for-ai")
	cobra.OnInitialize(setupLogs)

	rootCmd.AddCommand(driveCmd.DriveCmd)
	rootCmd.AddCommand(mailCmd.MailCmd)
}
