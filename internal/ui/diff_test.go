package ui

import (
	"strings"
	"testing"
)

func TestRenderDiffShowsOnlyChangedRegion(t *testing.T) {
	before := "min_version = \"2026.8.15\"\n\n[tools]\nnode = \"lts\"\n\n[dotfiles]\n"
	after := "min_version = \"2026.8.15\"\n\n[tools]\nnode = \"lts\"\nruby = \"3.4\"\n\n[dotfiles]\n"
	got := RenderDiff("mise.toml", before, after)
	for _, fragment := range []string{
		"--- mise.toml", "+++ mise.toml (proposto)",
		" node = \"lts\"", "+ruby = \"3.4\"", " [dotfiles]",
	} {
		if !strings.Contains(got, fragment) {
			t.Errorf("diff does not contain %q:\n%s", fragment, got)
		}
	}
	if strings.Contains(got, "-ruby") {
		t.Fatalf("added line was rendered as removal:\n%s", got)
	}
}

func TestRenderDiffReturnsEmptyForEqualContent(t *testing.T) {
	if got := RenderDiff("mise.toml", "same\n", "same\n"); got != "" {
		t.Fatalf("equal diff = %q", got)
	}
}
