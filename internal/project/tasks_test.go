package project

import (
	"strings"
	"testing"
)

func TestTaskDiscoveryKeepsNativeSourcesAndCommands(t *testing.T) {
	tasks, err := DecodeTasks([]byte(`[
		{"name":"test","aliases":["t"],"source":"/app/mise.toml","file":null,"run":["go test ./...",{"task":"lint"}]},
		{"name":"build","description":"Build app","source":"/global/mise.toml","file":"/global/tasks/build","global":true}
	]`))
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 || tasks[0].Name != "build" || !tasks[0].Global || tasks[0].EditPath() != "/global/tasks/build" {
		t.Fatalf("tasks = %#v", tasks)
	}
	if tasks[1].EditPath() != "/app/mise.toml" || !strings.Contains(tasks[1].Command(), "go test ./...") || !strings.Contains(tasks[1].Command(), `"lint"`) {
		t.Fatalf("TOML task = %#v", tasks[1])
	}
	if !tasks[1].Matches("t") || !tasks[1].Matches("test") || tasks[1].Matches("other") {
		t.Fatal("native task aliases were not preserved")
	}
	if _, err := DecodeTasks([]byte("not JSON")); err == nil {
		t.Fatal("invalid mise output accepted")
	}
}

func TestTaskLineFindsNativeTOMLLayoutsWithoutMatchingScriptContents(t *testing.T) {
	for _, tc := range []struct {
		name string
		data string
		line int
	}{
		{"test", "# [tasks.test]\n[tasks.test]\nrun = 'true'\n", 2},
		{"db:test", "[tools]\nnode = 'lts'\n[tasks.'db:test']\nrun = 'true'\n", 3},
		{"build", "[tasks]\n# build\nbuild = 'make'\n", 3},
		{"check", "# comment\ntasks.check.run = 'true'\n", 2},
		{"test", "[tasks.example]\nrun = '''\n[tasks.test]\n'''\n[tasks.test]\nrun = 'true'\n", 5},
		{"absent", "[tasks.test]\nrun = 'true'\n", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := TaskLine([]byte(tc.data), tc.name); got != tc.line {
				t.Fatalf("line = %d, want %d", got, tc.line)
			}
		})
	}
}
