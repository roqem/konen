package compat

import (
	"strings"
	"testing"
)

func TestTOMLVersionDistinguishesMissingAndDeclaredValues(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		version int
		present bool
	}{
		{name: "legacy", data: "path = '/tmp'\n", version: 0, present: false},
		{name: "zero", data: "version = 0\n", version: 0, present: true},
		{name: "current", data: "version = 3\n", version: 3, present: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			version, present, err := TOMLVersion([]byte(test.data))
			if err != nil {
				t.Fatal(err)
			}
			if version != test.version || present != test.present {
				t.Fatalf("TOMLVersion() = %d, %t", version, present)
			}
		})
	}
}

func TestVersionErrorGuidesMigrationAndUpdate(t *testing.T) {
	old := VersionError{Format: "o formato", Found: 1, Current: 2, Migratable: true}
	if !strings.Contains(old.Error(), "konen migrate --dry-run") {
		t.Fatalf("old error = %q", old.Error())
	}
	future := VersionError{Format: "o formato", Found: 3, Current: 2}
	if !strings.Contains(future.Error(), "konen update") {
		t.Fatalf("future error = %q", future.Error())
	}
}
