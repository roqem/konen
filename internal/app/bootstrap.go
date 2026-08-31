package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/roqem/konen/internal/ui"
)

type commaListFlag []string

func (values *commaListFlag) Set(input string) error {
	for _, value := range strings.Split(input, ",") {
		value = strings.TrimSpace(value)
		if value != "" && !containsString(*values, value) {
			*values = append(*values, value)
		}
	}
	return nil
}

func (values *commaListFlag) String() string {
	return strings.Join(*values, ",")
}

type applyPartSpec struct {
	part  ui.ApplyPart
	paths [][]string
}

var applyPartSpecs = []applyPartSpec{
	{
		part:  ui.ApplyPart{Key: "packages", Label: "Pacotes", Description: "pacotes do sistema"},
		paths: [][]string{{"bootstrap", "packages"}},
	},
	{
		part:  ui.ApplyPart{Key: "repos", Label: "Repositórios", Description: "checkouts Git do ambiente"},
		paths: [][]string{{"bootstrap", "repos"}},
	},
	{
		part:  ui.ApplyPart{Key: "dotfiles", Label: "Dotfiles", Description: "arquivos de configuração"},
		paths: [][]string{{"dotfiles"}},
	},
	{
		part:  ui.ApplyPart{Key: "tools", Label: "Ferramentas", Description: "linguagens e CLIs do mise"},
		paths: [][]string{{"tools"}},
	},
	{
		part:  ui.ApplyPart{Key: "task", Label: "Tarefas pessoais", Description: "instaladores e configurações próprias"},
		paths: [][]string{{"tasks", "bootstrap"}},
	},
}

func (a *App) chooseApplyParts() ([]string, error) {
	stateDir, err := a.loadState()
	if err != nil {
		return nil, err
	}
	contents, err := os.ReadFile(filepath.Join(stateDir, "mise.toml"))
	if err != nil {
		return nil, err
	}
	parts, err := declaredApplyParts(contents)
	if err != nil {
		return nil, fmt.Errorf("mise.toml inválido: %w", err)
	}
	if len(parts) == 0 {
		return nil, errors.New("o estado não declara nenhuma etapa aplicável")
	}
	return a.options.Prompter.ChooseApplyParts(parts)
}

func declaredApplyParts(contents []byte) ([]ui.ApplyPart, error) {
	var document map[string]any
	if err := toml.Unmarshal(contents, &document); err != nil {
		return nil, err
	}
	var parts []ui.ApplyPart
	for _, spec := range applyPartSpecs {
		for _, path := range spec.paths {
			if configuredValue(valueAtPath(document, path)) {
				parts = append(parts, spec.part)
				break
			}
		}
	}
	return parts, nil
}

func valueAtPath(document map[string]any, path []string) any {
	var current any = document
	for _, key := range path {
		mapping, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current, ok = mapping[key]
		if !ok {
			return nil
		}
	}
	return current
}

func configuredValue(value any) bool {
	switch typed := value.(type) {
	case nil:
		return false
	case map[string]any:
		return len(typed) > 0
	case []any:
		return len(typed) > 0
	case string:
		return strings.TrimSpace(typed) != ""
	default:
		return true
	}
}

func validateApplyParts(parts []string) error {
	known := make(map[string]bool, len(applyPartSpecs))
	for _, spec := range applyPartSpecs {
		known[spec.part.Key] = true
	}
	var invalid []string
	for _, part := range parts {
		if !known[part] {
			invalid = append(invalid, part)
		}
	}
	if len(invalid) == 0 {
		return nil
	}
	sort.Strings(invalid)
	return fmt.Errorf(
		"etapa desconhecida: %s (use packages, repos, dotfiles, tools ou task)",
		strings.Join(invalid, ", "),
	)
}

func applyPartLabels(parts []string) []string {
	labels := make([]string, 0, len(parts))
	for _, part := range parts {
		label := part
		for _, spec := range applyPartSpecs {
			if spec.part.Key == part {
				label = spec.part.Label
				break
			}
		}
		labels = append(labels, label)
	}
	return labels
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
