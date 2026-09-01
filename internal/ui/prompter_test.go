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

func TestProjectActionFieldsRejectIncompleteAndAmbiguousValues(t *testing.T) {
	if err := validateProjectTask(""); err == nil || !strings.Contains(err.Error(), "não pode ser vazia") {
		t.Fatalf("empty task error = %v", err)
	}
	if err := validateProjectActionName(nil)("test"); err != nil {
		t.Fatalf("valid action name error = %v", err)
	}
	if err := validateProjectActionName([]ProjectActionAnswer{{Name: "test"}})("test"); err == nil {
		t.Fatal("duplicate action name was accepted")
	}
	if err := validateTabAction([]ProjectActionAnswer{{Name: "test"}})("missing"); err == nil {
		t.Fatal("unknown tab action was accepted")
	}
	action := "test"
	if err := validateDirectCommand(&action)("go test ./..."); err == nil || !strings.Contains(err.Error(), "não ambos") {
		t.Fatalf("action plus command error = %v", err)
	}
}
