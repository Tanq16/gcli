package ui

import (
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/rs/zerolog/log"
)

var debug bool

var (
	infoStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("12")) // bright blue
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10")) // bright green
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))  // bright red
	warnStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("11")) // bright yellow
)

// Init sets the debug mode for the UI package
func Init(debugMode bool) {
	debug = debugMode
}

// PrintInfo prints an info message in blue
func PrintInfo(msg string) {
	if debug {
		log.Info().Msg(msg)
	} else {
		fmt.Println(infoStyle.Render("→ " + msg))
	}
}

// PrintSuccess prints a success message in green
func PrintSuccess(msg string) {
	if debug {
		log.Info().Msg(msg)
	} else {
		fmt.Println(successStyle.Render("✓ " + msg))
	}
}

// PrintError prints an error message in red (does not exit)
func PrintError(msg string, err error) {
	if debug && err != nil {
		log.Error().Err(err).Msg(msg)
	} else {
		fmt.Println(errorStyle.Render("✗ " + msg))
	}
}

// PrintFatal prints an error message and exits
func PrintFatal(msg string, err error) {
	if debug && err != nil {
		log.Error().Err(err).Msg(msg)
	} else {
		fmt.Println(errorStyle.Render("✗ " + msg))
	}
	os.Exit(1)
}

// PrintWarn prints a warning message in yellow
func PrintWarn(msg string, err error) {
	if debug && err != nil {
		log.Warn().Err(err).Msg(msg)
	} else {
		fmt.Println(warnStyle.Render("! " + msg))
	}
}

// PrintGeneric prints plain text without styling
func PrintGeneric(msg string) {
	fmt.Println(msg)
}

// FormatSize converts bytes to human-readable size
func FormatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
