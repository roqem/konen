package ui

import "fmt"

func CommandLabel(command, description string, width int) string {
	if width < len(command) {
		width = len(command)
	}
	return fmt.Sprintf("%-*s — %s", width, command, description)
}
