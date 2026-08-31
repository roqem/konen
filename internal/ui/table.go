package ui

import (
	"strings"
	"unicode/utf8"
)

// RenderTable renders a lightweight terminal table: one header row with
// visible column separators, one horizontal rule, and an unboxed body.
func RenderTable(headers []string, rows [][]string) string {
	if len(headers) == 0 {
		return ""
	}

	widths := make([]int, len(headers))
	for index, header := range headers {
		widths[index] = utf8.RuneCountInString(header)
	}
	for _, row := range rows {
		for index := 0; index < len(row) && index < len(widths); index++ {
			if width := utf8.RuneCountInString(row[index]); width > widths[index] {
				widths[index] = width
			}
		}
	}

	var output strings.Builder
	writeRow(&output, headers, widths, " | ")
	ruleWidth := 3 * (len(headers) - 1)
	for _, width := range widths {
		ruleWidth += width
	}
	output.WriteString(strings.Repeat("─", ruleWidth))
	output.WriteByte('\n')
	for _, row := range rows {
		writeRow(&output, row, widths, "   ")
	}
	return output.String()
}

func writeRow(output *strings.Builder, values []string, widths []int, separator string) {
	for index, width := range widths {
		if index > 0 {
			output.WriteString(separator)
		}
		value := ""
		if index < len(values) {
			value = values[index]
		}
		output.WriteString(value)
		if index < len(widths)-1 {
			output.WriteString(strings.Repeat(" ", width-utf8.RuneCountInString(value)))
		}
	}
	output.WriteByte('\n')
}
