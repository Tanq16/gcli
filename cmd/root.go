package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	calCmd "github.com/tanq16/gdrive/cmd/cal-cmd"
	driveCmd "github.com/tanq16/gdrive/cmd/drive-cmd"
	mailCmd "github.com/tanq16/gdrive/cmd/mail-cmd"
	u "github.com/tanq16/gdrive/utils"
)

var AppVersion = "dev-build"
var debugFlag bool
var forAIFlag bool

var rootCmd = &cobra.Command{
	Use:     "gcli",
	Short:   "CLI tool for Google Drive, Gmail, and Calendar",
	Version: AppVersion,
	CompletionOptions: cobra.CompletionOptions{
		HiddenDefaultCmd: true,
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func setupLogs() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	output := zerolog.ConsoleWriter{
		Out:        os.Stdout,
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
	rootCmd.AddCommand(calCmd.CalCmd)
}
