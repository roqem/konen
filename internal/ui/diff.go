package ui

import (
	"fmt"
	"strings"
)

// RenderDiff renders the smallest changed region with up to three surrounding
// context lines. It is intentionally small because guided state mutations
// change one declaration at a time.
func RenderDiff(path, before, after string) string {
	if before == after {
		return ""
	}
	beforeLines := splitDiffLines(before)
	afterLines := splitDiffLines(after)

	prefix := 0
	for prefix < len(beforeLines) && prefix < len(afterLines) && beforeLines[prefix] == afterLines[prefix] {
		prefix++
	}
	suffix := 0
	for suffix < len(beforeLines)-prefix && suffix < len(afterLines)-prefix &&
		beforeLines[len(beforeLines)-1-suffix] == afterLines[len(afterLines)-1-suffix] {
		suffix++
	}

	contextStart := max(0, prefix-3)
	beforeChangeEnd := len(beforeLines) - suffix
	afterChangeEnd := len(afterLines) - suffix
	contextSuffix := min(3, suffix)
	beforeEnd := beforeChangeEnd + contextSuffix
	afterEnd := afterChangeEnd + contextSuffix

	var output strings.Builder
	fmt.Fprintf(&output, "--- %s\n+++ %s (proposto)\n", path, path)
	fmt.Fprintf(&output, "@@ -%d,%d +%d,%d @@\n",
		contextStart+1, beforeEnd-contextStart,
		contextStart+1, afterEnd-contextStart,
	)
	for _, line := range beforeLines[contextStart:prefix] {
		fmt.Fprintf(&output, " %s\n", line)
	}
	for _, line := range beforeLines[prefix:beforeChangeEnd] {
		fmt.Fprintf(&output, "-%s\n", line)
	}
	for _, line := range afterLines[prefix:afterChangeEnd] {
		fmt.Fprintf(&output, "+%s\n", line)
	}
	for _, line := range afterLines[afterChangeEnd:afterEnd] {
		fmt.Fprintf(&output, " %s\n", line)
	}
	return output.String()
}

func splitDiffLines(value string) []string {
	value = strings.TrimSuffix(value, "\n")
	if value == "" {
		return nil
	}
	return strings.Split(value, "\n")
}
