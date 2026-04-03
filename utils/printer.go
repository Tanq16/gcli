package utils

import (
	"fmt"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/rs/zerolog/log"
)

var (
	infoStyle    = lipgloss.NewStyle().Foreground(lipgloss.ANSIColor(12)) // bright blue
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.ANSIColor(10)) // bright green
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.ANSIColor(9))  // bright red
	warnStyle    = lipgloss.NewStyle().Foreground(lipgloss.ANSIColor(11)) // bright yellow
)

// PrintInfo prints an info message in blue
func PrintInfo(msg string) {
	if GlobalDebugFlag {
		log.Info().Str("package", "utils").Msg(msg)
	} else if GlobalForAIFlag {
		fmt.Println("[INFO] " + msg)
	} else {
		fmt.Println(infoStyle.Render("→ " + msg))
	}
}

// PrintSuccess prints a success message in green
func PrintSuccess(msg string) {
	if GlobalDebugFlag {
		log.Info().Str("package", "utils").Msg(msg)
	} else if GlobalForAIFlag {
		fmt.Println("[OK] " + msg)
	} else {
		fmt.Println(successStyle.Render("✓ " + msg))
	}
}

// PrintError prints an error message in red (does not exit)
// Only --debug shows the underlying error; human and AI modes show only the friendly message
func PrintError(msg string, err error) {
	if GlobalDebugFlag {
		if err != nil {
			log.Error().Str("package", "utils").Err(err).Msg(msg)
		} else {
			log.Error().Str("package", "utils").Msg(msg)
		}
	} else if GlobalForAIFlag {
		fmt.Println("[ERROR] " + msg)
	} else {
		fmt.Println(errorStyle.Render("✗ " + msg))
	}
}

// PrintFatal prints an error message and exits
// Only --debug shows the underlying error; human and AI modes show only the friendly message
func PrintFatal(msg string, err error) {
	if GlobalDebugFlag {
		if err != nil {
			log.Error().Str("package", "utils").Err(err).Msg(msg)
		} else {
			log.Error().Str("package", "utils").Msg(msg)
		}
	} else if GlobalForAIFlag {
		fmt.Println("[ERROR] " + msg)
	} else {
		fmt.Println(errorStyle.Render("✗ " + msg))
	}
	os.Exit(1)
}

// PrintWarn prints a warning message in yellow
// Only --debug shows the underlying error; human and AI modes show only the friendly message
func PrintWarn(msg string, err error) {
	if GlobalDebugFlag {
		if err != nil {
			log.Warn().Str("package", "utils").Err(err).Msg(msg)
		} else {
			log.Warn().Str("package", "utils").Msg(msg)
		}
	} else if GlobalForAIFlag {
		fmt.Println("[WARN] " + msg)
	} else {
		fmt.Println(warnStyle.Render("! " + msg))
	}
}

// PrintRunning prints a transient running indicator (cleared later by ClearLines)
func PrintRunning(msg string) {
	if GlobalDebugFlag {
		log.Info().Str("package", "utils").Msg(msg)
	} else if GlobalForAIFlag {
		fmt.Println("[RUNNING] " + msg)
	} else {
		fmt.Println(infoStyle.Render("↻ " + msg))
	}
}

// PrintIndentedSuccess prints an indented success message
func PrintIndentedSuccess(msg string) {
	if GlobalDebugFlag {
		log.Info().Str("package", "utils").Msg(msg)
	} else if GlobalForAIFlag {
		fmt.Println("[OK]   " + msg)
	} else {
		fmt.Println(successStyle.Render("  ✓ " + msg))
	}
}

// PrintIndentedError prints an indented error message
func PrintIndentedError(msg string, err error) {
	if GlobalDebugFlag {
		if err != nil {
			log.Error().Str("package", "utils").Err(err).Msg(msg)
		} else {
			log.Error().Str("package", "utils").Msg(msg)
		}
	} else if GlobalForAIFlag {
		fmt.Println("[ERROR]   " + msg)
	} else {
		fmt.Println(errorStyle.Render("  ✗ " + msg))
	}
}

// PrintIndentedWarn prints an indented warning message
func PrintIndentedWarn(msg string, err error) {
	if GlobalDebugFlag {
		if err != nil {
			log.Warn().Str("package", "utils").Err(err).Msg(msg)
		} else {
			log.Warn().Str("package", "utils").Msg(msg)
		}
	} else if GlobalForAIFlag {
		fmt.Println("[WARN]   " + msg)
	} else {
		fmt.Println(warnStyle.Render("  ! " + msg))
	}
}

// PrintProgress prints a braille-dot progress bar (meant to be called from a goroutine ticker)
func PrintProgress(label string, percent int) {
	if GlobalDebugFlag {
		log.Info().Str("package", "utils").Int("percent", percent).Msg(label)
		return
	}
	if GlobalForAIFlag {
		fmt.Printf("[PROGRESS] %s: %d%%\n", label, percent)
		return
	}
	const barWidth = 10
	filled := barWidth * percent / 100
	empty := barWidth - filled
	bar := strings.Repeat("⣿", filled) + strings.Repeat("⣀", empty)
	fmt.Println(infoStyle.Render(fmt.Sprintf("  ↻ %s: %s %d%%", label, bar, percent)))
}

// ClearLines moves the cursor up n lines and clears them (human mode only)
// In AI and debug modes this is a no-op so all output persists
func ClearLines(n int) {
	if GlobalDebugFlag || GlobalForAIFlag {
		return
	}
	for range n {
		fmt.Print("\033[A\033[2K")
	}
}

// ClearPreviousLine clears the single previous line (human mode only)
func ClearPreviousLine() {
	ClearLines(1)
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
