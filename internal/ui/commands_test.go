package ui

import (
	"errors"
	"fmt"
	"slices"
	"testing"

	"charm.land/huh/v2"
)

func TestCommandLabelAlignsCommandAndDescription(t *testing.T) {
	got := CommandLabel("dev", "abrir um projeto", 8)
	if got != "dev      — abrir um projeto" {
		t.Fatalf("CommandLabel() = %q", got)
	}
}

func TestMenuAndFormExitKeys(t *testing.T) {
	menuKeys := menuKeyMap().Quit.Keys()
	for _, key := range []string{"ctrl+c", "esc", "q", "Q"} {
		if !slices.Contains(menuKeys, key) {
			t.Errorf("menu exit keys are missing %q: %#v", key, menuKeys)
		}
	}

	formKeys := cancelKeyMap().Quit.Keys()
	for _, key := range []string{"ctrl+c", "esc"} {
		if !slices.Contains(formKeys, key) {
			t.Errorf("form exit keys are missing %q: %#v", key, formKeys)
		}
	}
	if slices.Contains(formKeys, "q") {
		t.Fatalf("q must remain available when typing into forms: %#v", formKeys)
	}
}

func TestIsUserAborted(t *testing.T) {
	if !IsUserAborted(fmt.Errorf("wrapped: %w", huh.ErrUserAborted)) {
		t.Fatal("wrapped user abort was not recognized")
	}
	if IsUserAborted(errors.New("different error")) {
		t.Fatal("unrelated error was recognized as user abort")
	}
}
