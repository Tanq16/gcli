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

func PrintInfo(msg string) {
	if GlobalDebugFlag {
		log.Info().Str("package", "utils").Msg(msg)
	} else if GlobalForAIFlag {
		fmt.Println("[INFO] " + msg)
	} else {
		fmt.Println(infoStyle.Render("→ " + msg))
	}
}

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

func PrintRunning(msg string) {
	if GlobalDebugFlag {
		log.Info().Str("package", "utils").Msg(msg)
	} else if GlobalForAIFlag {
		fmt.Println("[RUNNING] " + msg)
	} else {
		fmt.Println(infoStyle.Render("↻ " + msg))
	}
}

func PrintIndentedRunning(msg string) {
	if GlobalDebugFlag {
		log.Info().Str("package", "utils").Msg(msg)
	} else if GlobalForAIFlag {
		fmt.Println("[RUNNING]   " + msg)
	} else {
		fmt.Println(infoStyle.Render("  ↻ " + msg))
	}
}

func PrintIndentedSuccess(msg string) {
	if GlobalDebugFlag {
		log.Info().Str("package", "utils").Msg(msg)
	} else if GlobalForAIFlag {
		fmt.Println("[OK]   " + msg)
	} else {
		fmt.Println(successStyle.Render("  ✓ " + msg))
	}
}

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

func ClearLines(n int) {
	if GlobalDebugFlag || GlobalForAIFlag {
		return
	}
	for range n {
		fmt.Print("\033[A\033[2K")
	}
}

func ClearPreviousLine() {
	ClearLines(1)
}

func PrintGeneric(msg string) {
	fmt.Println(msg)
}

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
