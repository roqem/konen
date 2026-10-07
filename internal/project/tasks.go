package project

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2/unstable"
)

// Task describes a native mise task. Its implementation stays in Source/File.
type Task struct {
	Name        string            `json:"name"`
	Aliases     []string          `json:"aliases"`
	Description string            `json:"description"`
	Source      string            `json:"source"`
	File        string            `json:"file"`
	Global      bool              `json:"global"`
	Run         []json.RawMessage `json:"run"`
}

func DecodeTasks(data []byte) ([]Task, error) {
	var tasks []Task
	if err := json.Unmarshal(data, &tasks); err != nil {
		return nil, fmt.Errorf("lista de tarefas inválida recebida do mise: %w", err)
	}
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].Name < tasks[j].Name })
	return tasks, nil
}

func (t Task) EditPath() string {
	if t.File != "" {
		return t.File
	}
	return t.Source
}

func (t Task) Matches(name string) bool {
	if t.Name == name {
		return true
	}
	for _, alias := range t.Aliases {
		if alias == name {
			return true
		}
	}
	return false
}

func (t Task) Command() string {
	if t.File != "" {
		return t.File
	}
	commands := make([]string, 0, len(t.Run))
	for _, raw := range t.Run {
		var command string
		if json.Unmarshal(raw, &command) == nil {
			commands = append(commands, command)
		} else {
			commands = append(commands, string(raw))
		}
	}
	return strings.Join(commands, " ; ")
}

// TaskLine locates table, dotted and quoted task keys without matching headers
// inside comments or multiline strings. Unknown layouts open at the beginning.
func TaskLine(data []byte, name string) int {
	var parser unstable.Parser
	parser.Reset(data)
	var table []string
	for parser.NextExpression() {
		node := parser.Expression()
		if node.Kind != unstable.Table && node.Kind != unstable.ArrayTable && node.Kind != unstable.KeyValue {
			continue
		}
		var key []string
		iterator := node.Key()
		var offset uint32
		for iterator.Next() {
			if len(key) == 0 {
				offset = iterator.Node().Raw.Offset
			}
			key = append(key, string(iterator.Node().Data))
		}
		if node.Kind == unstable.Table || node.Kind == unstable.ArrayTable {
			table = key
		} else {
			key = append(append([]string(nil), table...), key...)
		}
		if len(key) >= 2 && key[0] == "tasks" && key[1] == name {
			return bytes.Count(data[:offset], []byte("\n")) + 1
		}
	}
	return 1
}
