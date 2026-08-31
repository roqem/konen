package ui

import "testing"

func TestDefaultProjectTabDoesNotAssumeAnInstalledEditor(t *testing.T) {
	tab := defaultProjectTab()
	if tab.Title != "Terminal" {
		t.Fatalf("default title = %q, want Terminal", tab.Title)
	}
	if tab.Command != "" {
		t.Fatalf("default command = %q, want an interactive shell", tab.Command)
	}
}
