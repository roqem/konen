package ui

import (
	"strings"
	"testing"

	"charm.land/huh/v2"
)

func TestDefaultProjectTabDoesNotAssumeAnInstalledEditor(t *testing.T) {
	tab := defaultProjectTab()
	if tab.Title != "Terminal" {
		t.Fatalf("default title = %q, want Terminal", tab.Title)
	}
	if tab.Command != "" {
		t.Fatalf("default command = %q, want an interactive shell", tab.Command)
	}
}

func TestApplyPartsFieldRendersASingleOption(t *testing.T) {
	selected := []string{"dotfiles"}
	field := applyPartsField([]huh.Option[string]{
		huh.NewOption("Dotfiles — arquivos de configuração", "dotfiles").Selected(true),
	}, &selected)
	field.Width(60)

	view := field.View()
	if !strings.Contains(view, "Dotfiles") {
		t.Fatalf("single-option selector rendered an empty viewport:\n%s", view)
	}
}
