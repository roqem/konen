package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/roqem/konen/internal/state"
	"github.com/roqem/konen/internal/ui"
)

type statusRow struct {
	kind          string
	item          string
	configuration string
	state         string
	part          string
}

func (a *App) runStatus(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("status não aceita argumentos")
	}
	stateDir, misePath, err := a.loadTrustedMise(ctx)
	if err != nil {
		return err
	}
	output, err := a.options.Runner.OutputEnv(
		ctx, stateDir, miseStateEnvironment(stateDir), misePath,
		"-C", stateDir, "bootstrap", "status", "--json",
	)
	if err != nil {
		return fmt.Errorf("mise: %w", err)
	}
	formatted, err := formatMiseStatusWithState([]byte(output), stateDir)
	if err != nil {
		return fmt.Errorf("status do mise inválido: %w", err)
	}
	fmt.Fprint(a.options.Out, formatted)
	fmt.Fprint(a.options.Out, renderGitBackup(a.inspectGitBackup(ctx, stateDir), stateDir))
	return nil
}

func formatMiseStatus(data []byte) (string, error) {
	return formatMiseStatusWithState(data, "")
}

func formatMiseStatusWithState(data []byte, stateDir string) (string, error) {
	rows, err := parseMiseStatusRows(data, stateDir)
	if err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return "Nenhum item declarado no estado.\n", nil
	}

	tableRows := make([][]string, 0, len(rows))
	for _, row := range rows {
		tableRows = append(tableRows, []string{row.kind, row.item, row.configuration, row.state})
	}
	return ui.RenderTable([]string{"Tipo", "Item", "Configuração", "Estado"}, tableRows), nil
}

func parseMiseStatusRows(data []byte, stateDir string) ([]statusRow, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var root map[string]any
	if err := decoder.Decode(&root); err != nil {
		return nil, err
	}

	var rows []statusRow
	for _, key := range orderedKeys(root, statusRootOrder) {
		collectStatusRows([]string{key}, root[key], &rows)
	}
	if stateDir != "" {
		personal, err := personalScriptRows(stateDir)
		if err != nil {
			return nil, err
		}
		rows = append(rows, personal...)
	}
	return rows, nil
}

func personalScriptRows(stateDir string) ([]statusRow, error) {
	_, _, files, err := state.ExecutionDigest(stateDir)
	if err != nil {
		return nil, err
	}
	var rows []statusRow
	for _, relative := range files {
		if relative == "mise.toml" || strings.HasSuffix(relative, "/.gitkeep") {
			continue
		}
		path := filepath.Join(stateDir, filepath.FromSlash(relative))
		info, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		executable := info.Mode()&0o111 != 0
		declaredState := "disponível"
		if !executable {
			declaredState = "não executável"
		}
		switch {
		case state.IsPersonalCommand(relative):
			name := strings.TrimPrefix(relative, "scripts/bin/")
			rows = append(rows, statusRow{
				kind: "Comando pessoal", item: name,
				configuration: "Fonte: " + relative, state: declaredState, part: "task",
			})
		default:
			name, ok := state.TaskName(relative)
			if !ok {
				continue
			}
			kind := "Tarefa pessoal"
			if strings.HasPrefix(name, "install:") {
				kind = "Instalador pessoal"
			}
			rows = append(rows, statusRow{
				kind: kind, item: name,
				configuration: "Fonte: " + relative, state: declaredState, part: "task",
			})
		}
	}
	return rows, nil
}

func collectStatusRows(path []string, value any, rows *[]statusRow) {
	switch typed := value.(type) {
	case nil:
		return
	case []any:
		for index, item := range typed {
			if record, ok := item.(map[string]any); ok {
				*rows = append(*rows, statusRecord(path, record, fmt.Sprintf("#%d", index+1)))
				continue
			}
			collectStatusRows(append(path, fmt.Sprintf("#%d", index+1)), item, rows)
		}
	case map[string]any:
		if len(typed) == 0 {
			return
		}
		if statusRecordLike(typed) {
			fallback := path[len(path)-1]
			recordPath := path
			if !statusRecordHasIdentity(typed) {
				recordPath = path[:len(path)-1]
			}
			*rows = append(*rows, statusRecord(recordPath, typed, fallback))
			return
		}
		for _, key := range orderedKeys(typed, nil) {
			if len(path) >= 1 && path[0] == "packages" && key == "available" {
				continue
			}
			collectStatusRows(append(path, key), typed[key], rows)
		}
	default:
		if len(path) == 0 {
			return
		}
		kindPath := path[:len(path)-1]
		*rows = append(*rows, statusRow{
			kind:          statusKind(kindPath),
			item:          humanize(path[len(path)-1]),
			configuration: statusValue(typed),
			state:         "—",
			part:          statusApplyPart(path),
		})
	}
}

func statusRecordLike(record map[string]any) bool {
	for key := range record {
		if isStatusIdentity(key) || key == "state" || key == "status" {
			return true
		}
	}
	return false
}

func statusRecordHasIdentity(record map[string]any) bool {
	for key := range record {
		if isStatusIdentity(key) {
			return true
		}
	}
	return false
}

func statusRecord(path []string, record map[string]any, fallback string) statusRow {
	identityKey := ""
	for _, key := range statusIdentityOrder {
		if _, ok := record[key]; ok {
			identityKey = key
			break
		}
	}
	item := humanize(fallback)
	if identityKey != "" {
		item = statusValue(record[identityKey])
	}

	state := "—"
	for _, key := range []string{"state", "status"} {
		if value, ok := record[key]; ok {
			state = localizedState(statusValue(value))
			break
		}
	}

	var details []string
	for _, key := range orderedKeys(record, statusDetailOrder) {
		if key == identityKey || key == "state" || key == "status" {
			continue
		}
		if statusDetailEmpty(record[key]) {
			continue
		}
		if key == "path" && identityKey == "path_raw" {
			continue
		}
		if key == "origin" && equivalentRepositoryURL(record["url"], record[key]) {
			continue
		}
		details = append(details, fmt.Sprintf("%s: %s", statusField(key), statusDetailValue(key, record[key])))
	}
	configuration := "—"
	if len(details) > 0 {
		configuration = strings.Join(details, "; ")
	}
	return statusRow{
		kind: statusKind(path), item: item, configuration: configuration, state: state,
		part: statusApplyPart(path),
	}
}

func statusApplyPart(path []string) string {
	if len(path) == 0 {
		return ""
	}
	switch path[0] {
	case "packages", "repos", "dotfiles", "tools":
		return path[0]
	case "plugin_deps":
		return "tools"
	case "login_shell":
		return "user"
	default:
		return path[0]
	}
}

func orderedKeys(values map[string]any, preferred []string) []string {
	keys := make([]string, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, key := range preferred {
		if _, ok := values[key]; ok {
			keys = append(keys, key)
			seen[key] = true
		}
	}
	var remaining []string
	for key := range values {
		if !seen[key] {
			remaining = append(remaining, key)
		}
	}
	sort.Strings(remaining)
	return append(keys, remaining...)
}

func statusKind(path []string) string {
	joined := strings.Join(path, ".")
	if label, ok := statusKinds[joined]; ok {
		return label
	}
	if len(path) == 0 {
		return "Estado"
	}
	if len(path) == 3 && path[0] == "packages" && path[2] == "packages" {
		return fmt.Sprintf("Pacote (%s)", path[1])
	}
	parts := make([]string, len(path))
	for index, part := range path {
		if label, ok := statusPathParts[part]; ok {
			parts[index] = label
		} else {
			parts[index] = humanize(part)
		}
	}
	return strings.Join(parts, " · ")
}

func statusDetailEmpty(value any) bool {
	if value == nil {
		return true
	}
	text, ok := value.(string)
	return ok && strings.TrimSpace(text) == ""
}

func statusDetailValue(key string, value any) string {
	formatted := statusValue(value)
	if key == "current_sha" && len(formatted) > 12 {
		return formatted[:12]
	}
	return formatted
}

func equivalentRepositoryURL(left, right any) bool {
	leftText, leftOK := left.(string)
	rightText, rightOK := right.(string)
	if !leftOK || !rightOK {
		return false
	}
	normalize := func(value string) string { return strings.TrimSuffix(strings.TrimSpace(value), ".git") }
	return normalize(leftText) == normalize(rightText)
}

func statusField(key string) string {
	if label, ok := statusFields[key]; ok {
		return label
	}
	return humanize(key)
}

func statusValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return "—"
	case string:
		if typed == "" {
			return "—"
		}
		return typed
	case bool:
		if typed {
			return "sim"
		}
		return "não"
	case json.Number:
		return typed.String()
	default:
		encoded, err := json.Marshal(typed)
		if err != nil {
			return fmt.Sprint(typed)
		}
		return string(encoded)
	}
}

func localizedState(value string) string {
	if translated, ok := statusStates[value]; ok {
		return translated
	}
	return value
}

func humanize(value string) string {
	value = strings.ReplaceAll(value, "_", " ")
	value = strings.ReplaceAll(value, "-", " ")
	if value == "" {
		return "—"
	}
	runes := []rune(value)
	runes[0] = []rune(strings.ToUpper(string(runes[0])))[0]
	return string(runes)
}

func isStatusIdentity(key string) bool {
	for _, candidate := range statusIdentityOrder {
		if key == candidate {
			return true
		}
	}
	return false
}

var statusRootOrder = []string{
	"secrets", "packages", "accounts", "files", "services", "firewall",
	"compose", "repos", "dotfiles", "mise_shell_activate", "macos_defaults",
	"launchd", "systemd", "login_shell", "tools", "plugin_deps",
}

var statusIdentityOrder = []string{
	"target", "tool", "name", "package", "path_raw", "path", "id", "unit", "service",
	"repo", "user", "group", "shell", "key",
}

var statusDetailOrder = []string{
	"source", "mode", "requested_version", "resolved_version", "version",
	"installed_version", "installed", "enabled", "url", "ref", "current_ref",
	"current_sha", "current", "shell", "path", "origin", "reason",
}

var statusKinds = map[string]string{
	"dotfiles.files":         "Dotfile",
	"dotfiles.edits":         "Edição de dotfile",
	"tools":                  "Ferramenta",
	"packages":               "Pacote",
	"accounts":               "Conta",
	"files":                  "Arquivo de sistema",
	"services":               "Serviço",
	"firewall":               "Firewall",
	"compose":                "Docker Compose",
	"repos":                  "Repositório",
	"mise_shell_activate":    "Ativação do shell",
	"macos_defaults.entries": "Preferência do macOS",
	"launchd.agents":         "Agente launchd",
	"systemd.units":          "Unidade systemd",
	"login_shell":            "Shell de login",
	"plugin_deps":            "Plugin",
}

var statusPathParts = map[string]string{
	"packages":            "Pacote",
	"accounts":            "Conta",
	"files":               "Arquivo de sistema",
	"services":            "Serviço",
	"repos":               "Repositório",
	"tools":               "Ferramenta",
	"plugins":             "Plugin",
	"dotfiles":            "Dotfile",
	"mise_shell_activate": "Ativação do shell",
	"macos_defaults":      "Preferência do macOS",
	"plugin_deps":         "Plugin",
}

var statusFields = map[string]string{
	"source":            "Fonte",
	"mode":              "Modo",
	"requested_version": "Versão pedida",
	"resolved_version":  "Versão resolvida",
	"version":           "Versão",
	"installed_version": "Versão instalada",
	"installed":         "Instalado",
	"enabled":           "Habilitado",
	"available":         "Disponível",
	"url":               "URL",
	"ref":               "Referência",
	"current_ref":       "Referência atual",
	"current_sha":       "Commit atual",
	"current":           "Atual",
	"shell":             "Desejado",
	"path":              "Caminho",
	"origin":            "Origem atual",
	"path_raw":          "Caminho declarado",
	"reason":            "Motivo",
	"shell_listed":      "Shell registrado",
}

var statusStates = map[string]string{
	"applied":        "aplicado",
	"installed":      "instalado",
	"missing":        "ausente",
	"source_missing": "fonte ausente",
	"differs":        "diferente",
	"pending":        "pendente",
	"running":        "executando",
	"stopped":        "parado",
	"enabled":        "habilitado",
	"disabled":       "desabilitado",
	"current":        "atual",
	"set":            "configurado",
}
