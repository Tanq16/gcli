package utils

import (
	"fmt"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"golang.org/x/term"
)

var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.ANSIColor(15)).
			Padding(0, 1)

	cellStyle = lipgloss.NewStyle().
			Foreground(lipgloss.ANSIColor(7)).
			Padding(0, 1)

	borderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.ANSIColor(8))
)

// AI/debug get a lossless markdown table (never truncated); human mode is bounded to terminal width.
func PrintTable(headers []string, rows [][]string) {
	if GlobalForAIFlag || GlobalDebugFlag {
		printMarkdownTable(headers, rows)
		return
	}

	headers, rows = boundTable(headers, rows, terminalWidth())
	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(borderStyle).
		Headers(headers...).
		Rows(rows...).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return headerStyle
			}
			return cellStyle
		})

	PrintGeneric(t.Render())
}

func printMarkdownTable(headers []string, rows [][]string) {
	if len(headers) == 0 {
		return
	}
	fmt.Println("| " + strings.Join(escapeCells(headers), " | ") + " |")
	seps := make([]string, len(headers))
	for i := range seps {
		seps[i] = "---"
	}
	fmt.Println("| " + strings.Join(seps, " | ") + " |")
	for _, row := range rows {
		fmt.Println("| " + strings.Join(escapeCells(padRow(row, len(headers))), " | ") + " |")
	}
}

// Escape pipes and flatten newlines so a cell never breaks its markdown row.
func escapeCells(cells []string) []string {
	escaped := make([]string, len(cells))
	for i, cell := range cells {
		cell = strings.ReplaceAll(cell, "|", "\\|")
		cell = strings.ReplaceAll(cell, "\r\n", "\\n")
		cell = strings.ReplaceAll(cell, "\n", "\\n")
		escaped[i] = cell
	}
	return escaped
}

func padRow(row []string, n int) []string {
	if len(row) >= n {
		return row[:n]
	}
	out := make([]string, n)
	copy(out, row)
	return out
}

// max is display columns, not runes (CJK/emoji aware).
func truncateToWidth(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= max {
		return s
	}
	target := max - 1
	var b strings.Builder
	w := 0
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if w+rw > target {
			break
		}
		b.WriteRune(r)
		w += rw
	}
	return b.String() + "…"
}

func boundTable(headers []string, rows [][]string, maxWidth int) ([]string, [][]string) {
	n := len(headers)
	if n == 0 {
		return headers, rows
	}
	widths := make([]int, n)
	for i, h := range headers {
		widths[i] = lipgloss.Width(h)
	}
	for _, r := range rows {
		for i := 0; i < n && i < len(r); i++ {
			widths[i] = max(widths[i], lipgloss.Width(r[i]))
		}
	}

	const floor = 4
	overhead := 3*n + 1
	total := func() int {
		sum := overhead
		for _, w := range widths {
			sum += w
		}
		return sum
	}
	for total() > maxWidth {
		widest := -1
		for i, w := range widths {
			if w > floor && (widest == -1 || w > widths[widest]) {
				widest = i
			}
		}
		if widest == -1 {
			break
		}
		widths[widest]--
	}

	outHeaders := make([]string, n)
	for i, h := range headers {
		outHeaders[i] = truncateToWidth(h, widths[i])
	}
	outRows := make([][]string, len(rows))
	for ri, r := range rows {
		nr := make([]string, n)
		for i := range n {
			cell := ""
			if i < len(r) {
				cell = r[i]
			}
			nr[i] = truncateToWidth(cell, widths[i])
		}
		outRows[ri] = nr
	}
	return outHeaders, outRows
}

func terminalWidth() int {
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		return w
	}
	return 80
}
