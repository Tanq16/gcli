package utils

import (
	"errors"
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

// flattenErr renders the full wrapped error chain on a single line, collapsing
// newlines so one error is always one line in --for-ai output (spec §9.3).
func flattenErr(err error) string {
	if err == nil {
		return ""
	}
	s := err.Error()
	s = strings.ReplaceAll(s, "\r\n", "; ")
	s = strings.ReplaceAll(s, "\n", "; ")
	return strings.TrimSpace(s)
}

// aiError joins the human message with the full error chain in AI mode.
func aiError(prefix, msg string, err error) string {
	if err != nil {
		return prefix + msg + ": " + flattenErr(err)
	}
	return prefix + msg
}

func PrintInfo(msg string) {
	if GlobalDebugFlag {
		log.Info().Msg(msg)
	} else if GlobalForAIFlag {
		fmt.Println("[INFO] " + msg)
	} else {
		fmt.Println(infoStyle.Render("→ " + msg))
	}
}

func PrintSuccess(msg string) {
	if GlobalDebugFlag {
		log.Info().Msg(msg)
	} else if GlobalForAIFlag {
		fmt.Println("[OK] " + msg)
	} else {
		fmt.Println(successStyle.Render("✓ " + msg))
	}
}

// PrintError prints a diagnostic to stderr (does not exit). In --for-ai it
// appends the full wrapped error chain; human mode shows the friendly cause.
func PrintError(msg string, err error) {
	if GlobalDebugFlag {
		log.Error().Err(err).Msg(msg)
	} else if GlobalForAIFlag {
		fmt.Fprintln(os.Stderr, aiError("[ERROR] ", msg, err))
	} else {
		fmt.Fprintln(os.Stderr, errorStyle.Render("✗ "+humanMsg(msg, err)))
	}
}

// humanMsg appends the error's normalized cause to the message for the default
// human tier, so the user always sees why an operation failed (spec §9.3).
func humanMsg(msg string, err error) string {
	if err != nil {
		return msg + ": " + err.Error()
	}
	return msg
}

// PrintFatal prints a diagnostic and exits, deriving the exit code from an
// error implementing ExitCode() int (defaults to ExitGeneric).
func PrintFatal(msg string, err error) {
	PrintError(msg, err)
	os.Exit(exitCodeFor(err))
}

// PrintFatalCode prints a diagnostic and exits with an explicit code — used for
// failures classification cannot infer (usage, partial, auth-at-PreRun).
func PrintFatalCode(msg string, err error, code int) {
	PrintError(msg, err)
	os.Exit(code)
}

func exitCodeFor(err error) int {
	if err != nil {
		var coded interface{ ExitCode() int }
		if errors.As(err, &coded) {
			return coded.ExitCode()
		}
	}
	return ExitGeneric
}

func PrintWarn(msg string, err error) {
	if GlobalDebugFlag {
		log.Warn().Err(err).Msg(msg)
	} else if GlobalForAIFlag {
		fmt.Fprintln(os.Stderr, aiError("[WARN] ", msg, err))
	} else {
		fmt.Fprintln(os.Stderr, warnStyle.Render("! "+humanMsg(msg, err)))
	}
}

func PrintRunning(msg string) {
	if GlobalDebugFlag {
		log.Info().Msg(msg)
	} else if GlobalForAIFlag {
		fmt.Println("[RUNNING] " + msg)
	} else {
		fmt.Println(infoStyle.Render("↻ " + msg))
	}
}

func PrintIndentedRunning(msg string) {
	if GlobalDebugFlag {
		log.Info().Msg(msg)
	} else if GlobalForAIFlag {
		fmt.Println("[RUNNING]   " + msg)
	} else {
		fmt.Println(infoStyle.Render("  ↻ " + msg))
	}
}

func PrintIndentedSuccess(msg string) {
	if GlobalDebugFlag {
		log.Info().Msg(msg)
	} else if GlobalForAIFlag {
		fmt.Println("[OK]   " + msg)
	} else {
		fmt.Println(successStyle.Render("  ✓ " + msg))
	}
}

func PrintIndentedError(msg string, err error) {
	if GlobalDebugFlag {
		log.Error().Err(err).Msg(msg)
	} else if GlobalForAIFlag {
		fmt.Fprintln(os.Stderr, aiError("[ERROR]   ", msg, err))
	} else {
		fmt.Fprintln(os.Stderr, errorStyle.Render("  ✗ "+humanMsg(msg, err)))
	}
}

func PrintIndentedWarn(msg string, err error) {
	if GlobalDebugFlag {
		log.Warn().Err(err).Msg(msg)
	} else if GlobalForAIFlag {
		fmt.Fprintln(os.Stderr, aiError("[WARN]   ", msg, err))
	} else {
		fmt.Fprintln(os.Stderr, warnStyle.Render("  ! "+humanMsg(msg, err)))
	}
}

func PrintProgress(label string, percent int) {
	percent = min(max(percent, 0), 100)
	if GlobalDebugFlag {
		log.Info().Int("percent", percent).Msg(label)
		return
	}
	if GlobalForAIFlag {
		fmt.Printf("[PROGRESS] %s: %d%%\n", label, percent)
		return
	}
	fmt.Println(infoStyle.Render(fmt.Sprintf("  ↻ %s: %s %d%%", label, progressBar(percent), percent)))
}

func progressBar(percent int) string {
	percent = min(max(percent, 0), 100)
	const barWidth = 10
	filled := barWidth * percent / 100
	return strings.Repeat("⣿", filled) + strings.Repeat("⣀", barWidth-filled)
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
