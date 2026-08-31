package state

import (
	"strings"
	"testing"
)

func TestAddTableEntryCreatesSectionAndPreservesDocument(t *testing.T) {
	before := []byte("min_version = \"2026.8.15\"\n\n# keep this comment\n")
	after, exists, err := AddTableEntry(
		before, []string{"bootstrap", "packages"}, "apt:jq", `"latest"`,
	)
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("new entry reported as existing")
	}
	for _, fragment := range []string{
		"# keep this comment", "[bootstrap.packages]", `"apt:jq" = "latest"`,
	} {
		if !strings.Contains(string(after), fragment) {
			t.Fatalf("edited document is missing %q:\n%s", fragment, after)
		}
	}
}

func TestAddTableEntryUsesExistingSectionWithoutReformatting(t *testing.T) {
	before := []byte(`[bootstrap.packages]
"apt:git" = "latest"

[tools]
node = "lts"
`)
	after, exists, err := AddTableEntry(
		before, []string{"bootstrap", "packages"}, "apt:jq", `"latest"`,
	)
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("new entry reported as existing")
	}
	want := `"apt:git" = "latest"

"apt:jq" = "latest"
[tools]`
	if !strings.Contains(string(after), want) {
		t.Fatalf("entry was not inserted in the existing section:\n%s", after)
	}
}

func TestAddTableEntryDoesNotReplaceAnExistingDeclaration(t *testing.T) {
	before := []byte("[bootstrap.repos]\n\"~/src/app\" = { url = \"git@example.test:app.git\", ref = \"main\" }\n")
	after, exists, err := AddTableEntry(
		before, []string{"bootstrap", "repos"}, "~/src/app",
		`{ url = "https://example.test/other.git" }`,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatal("existing entry was not detected")
	}
	if string(after) != string(before) {
		t.Fatalf("existing entry changed:\n%s", after)
	}
}

func TestTOMLStringEscapesSpecialCharacters(t *testing.T) {
	got, err := TOMLString("quoted \"value\"\nnext")
	if err != nil {
		t.Fatal(err)
	}
	if got != `"quoted \"value\"\nnext"` {
		t.Fatalf("TOMLString() = %q", got)
	}
}

func TestAddTaskRunReferenceCreatesSequentialBootstrap(t *testing.T) {
	before := []byte("min_version = \"2026.8.15\"\n")
	after, exists, err := AddTaskRunReference(before, "install:chrome")
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("new task reference reported as existing")
	}
	want := `[tasks.bootstrap]
run = [
  { task = "install:chrome" },
]`
	if !strings.Contains(string(after), want) {
		t.Fatalf("bootstrap task was not created:\n%s", after)
	}
}

func TestAddTaskRunReferenceAppendsWithoutReformattingOrLosingComments(t *testing.T) {
	before := []byte(`[tasks.bootstrap]
# installers stay sequential
run = [
  { task = "install:chrome" }, # browser
	  { task = "install:docker" } # daemon
]

[tools]
node = "lts"
`)
	after, exists, err := AddTaskRunReference(before, "install:kitty")
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("new task reference reported as existing")
	}
	for _, fragment := range []string{
		"# installers stay sequential", `# browser`,
		`{ task = "install:docker" }, # daemon`, `{ task = "install:kitty" },`,
		"[tools]\nnode = \"lts\"",
	} {
		if !strings.Contains(string(after), fragment) {
			t.Fatalf("edited bootstrap is missing %q:\n%s", fragment, after)
		}
	}
}

func TestAddTaskRunReferenceSupportsSingleLineAndDetectsDuplicate(t *testing.T) {
	before := []byte("[tasks.bootstrap]\nrun = [{ task = \"install:chrome\" }]\n")
	after, exists, err := AddTaskRunReference(before, "install:kitty")
	if err != nil {
		t.Fatal(err)
	}
	if exists || !strings.Contains(string(after),
		`run = [{ task = "install:chrome" }, { task = "install:kitty" }]`) {
		t.Fatalf("single-line append = exists:%v\n%s", exists, after)
	}

	again, exists, err := AddTaskRunReference(after, "install:kitty")
	if err != nil {
		t.Fatal(err)
	}
	if !exists || string(again) != string(after) {
		t.Fatalf("duplicate changed the document: exists=%v\n%s", exists, again)
	}
}

func TestAddTaskRunReferenceRefusesIncompatibleRunDeclaration(t *testing.T) {
	before := []byte("[tasks.bootstrap]\nrun = \"mise run install:chrome\"\n")
	if _, _, err := AddTaskRunReference(before, "install:kitty"); err == nil ||
		!strings.Contains(err.Error(), "precisa ser uma lista") {
		t.Fatalf("incompatible run error = %v", err)
	}
}
