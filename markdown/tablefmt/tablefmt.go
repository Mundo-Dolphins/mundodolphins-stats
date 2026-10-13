package tablefmt

import (
	"fmt"
	"strings"
)

// Table represents a markdown table with headers and rows.
type Table struct {
	Headers []string
	Rows    [][]string
}

// RenderMarkdownTable renders a markdown table with padded columns.
func RenderMarkdownTable(t Table) (string, error) {
	if len(t.Headers) == 0 {
		return "", fmt.Errorf("table headers are required")
	}
	for i, row := range t.Rows {
		if len(row) != len(t.Headers) {
			return "", fmt.Errorf("row %d has %d columns, expected %d", i, len(row), len(t.Headers))
		}
	}

	widths := make([]int, len(t.Headers))
	for i, h := range t.Headers {
		widths[i] = cellWidth(h)
	}
	for _, row := range t.Rows {
		for i, cell := range row {
			w := cellWidth(cell)
			if w > widths[i] {
				widths[i] = w
			}
		}
	}

	var lines []string
	lines = append(lines, formatRow(t.Headers, widths))
	lines = append(lines, formatDivider(widths))
	for _, row := range t.Rows {
		lines = append(lines, formatRow(row, widths))
	}

	return strings.Join(lines, "\n"), nil
}

func formatRow(cells []string, widths []int) string {
	var b strings.Builder
	b.WriteString("|")
	for i, cell := range cells {
		content := sanitizeCell(cell)
		pad := widths[i] - cellWidth(content)
		b.WriteString(" ")
		b.WriteString(content)
		if pad > 0 {
			b.WriteString(strings.Repeat(" ", pad))
		}
		b.WriteString(" |")
	}
	return b.String()
}

func formatDivider(widths []int) string {
	var b strings.Builder
	b.WriteString("|")
	for _, w := range widths {
		if w < 3 {
			w = 3
		}
		b.WriteString(" ")
		b.WriteString(strings.Repeat("-", w))
		b.WriteString(" |")
	}
	return b.String()
}

func sanitizeCell(value string) string {
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "|", "\\|")
	value = strings.TrimSpace(value)
	return value
}

func cellWidth(value string) int {
	return len(sanitizeCell(value))
}
