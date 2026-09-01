package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/roqem/konen/internal/ui"
)

type applySummary struct {
	changed      []statusRow
	ready        []statusRow
	pendingScope []statusRow
	pendingAll   []statusRow
	taskRan      bool
	loginChanged bool
}

func (a *App) queryMiseStatusRows(ctx context.Context, stateDir, misePath string) ([]statusRow, error) {
	output, err := a.options.Runner.OutputEnv(
		ctx, stateDir, miseStateEnvironment(stateDir), misePath,
		"-C", stateDir, "bootstrap", "status", "--json",
	)
	if err != nil {
		return nil, fmt.Errorf("mise status: %w", err)
	}
	rows, err := parseMiseStatusRows([]byte(output), "")
	if err != nil {
		return nil, fmt.Errorf("status do mise inválido: %w", err)
	}
	return rows, nil
}

func taskWillRun(stateDir string, only []string) bool {
	if len(only) > 0 && !containsString(only, "task") {
		return false
	}
	contents, err := os.ReadFile(filepath.Join(stateDir, "mise.toml"))
	if err != nil {
		return false
	}
	parts, err := declaredApplyParts(contents)
	if err != nil {
		return false
	}
	for _, part := range parts {
		if part.Key == "task" {
			return true
		}
	}
	return false
}

func buildApplySummary(before, after []statusRow, only []string, taskRan bool) applySummary {
	summary := applySummary{taskRan: taskRan}
	beforeByKey := make(map[string][]statusRow)
	for _, row := range before {
		if rowInApplyScope(row, only) {
			key := statusRowKey(row)
			beforeByKey[key] = append(beforeByKey[key], row)
		}
	}

	for _, row := range after {
		if statusRowStateGroup(row) != "ready" {
			summary.pendingAll = append(summary.pendingAll, row)
		}
		if !rowInApplyScope(row, only) {
			continue
		}
		key := statusRowKey(row)
		previous, found := takeStatusRow(beforeByKey, key)
		switch {
		case statusRowStateGroup(row) != "ready":
			summary.pendingScope = append(summary.pendingScope, row)
		case !found || statusRowStateGroup(previous) != "ready" || previous.state != row.state || previous.configuration != row.configuration:
			summary.changed = append(summary.changed, row)
			if found && statusRowStateGroup(previous) != "ready" && (row.part == "user" || row.kind == "Shell de login") {
				summary.loginChanged = true
			}
		default:
			summary.ready = append(summary.ready, row)
		}
	}

	for _, remaining := range beforeByKey {
		for _, row := range remaining {
			if statusRowStateGroup(row) == "ready" {
				summary.ready = append(summary.ready, row)
				continue
			}
			summary.changed = append(summary.changed, row)
			if row.part == "user" || row.kind == "Shell de login" {
				summary.loginChanged = true
			}
		}
	}
	return summary
}

func takeStatusRow(rows map[string][]statusRow, key string) (statusRow, bool) {
	values := rows[key]
	if len(values) == 0 {
		return statusRow{}, false
	}
	row := values[0]
	if len(values) == 1 {
		delete(rows, key)
	} else {
		rows[key] = values[1:]
	}
	return row, true
}

func statusRowKey(row statusRow) string {
	return row.kind + "\x00" + row.item
}

func rowInApplyScope(row statusRow, only []string) bool {
	return len(only) == 0 || containsString(only, row.part)
}

func isConvergedState(state string) bool {
	switch state {
	case "aplicado", "instalado", "atual", "configurado", "executando", "habilitado":
		return true
	default:
		return false
	}
}

func summarizeStatusKinds(rows []statusRow) string {
	if len(rows) == 0 {
		return "—"
	}
	counts := make(map[string]int)
	for _, row := range rows {
		counts[row.kind]++
	}
	kinds := make([]string, 0, len(counts))
	for kind := range counts {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	parts := make([]string, 0, len(kinds))
	for _, kind := range kinds {
		parts = append(parts, fmt.Sprintf("%s: %d", kind, counts[kind]))
	}
	return strings.Join(parts, "; ")
}

func renderApplySummary(summary applySummary) string {
	rows := [][]string{
		{"Convergiram nesta execução", fmt.Sprint(len(summary.changed)), summarizeStatusKinds(summary.changed)},
		{"Já estavam prontos", fmt.Sprint(len(summary.ready)), summarizeStatusKinds(summary.ready)},
		{"Ainda pendentes nas etapas aplicadas", fmt.Sprint(len(summary.pendingScope)), summarizeStatusKinds(summary.pendingScope)},
	}
	if summary.taskRan {
		rows = append(rows, []string{
			"Etapa de tarefas pessoais", "1",
			"terminou sem erro; tarefas idempotentes não têm um estado final verificável",
		})
	}

	var output strings.Builder
	output.WriteByte('\n')
	output.WriteString(ui.RenderTable([]string{"Resumo da aplicação", "Itens", "Detalhes"}, rows))

	var next [][]string
	if len(summary.pendingAll) > 0 {
		next = append(next, []string{
			"Recursos pendentes", fmt.Sprint(len(summary.pendingAll)),
			summarizeStatusKinds(summary.pendingAll) + "; revise com `konen plan`",
		})
	}
	if summary.loginChanged {
		next = append(next, []string{
			"Nova sessão de login", "necessária",
			"saia e entre novamente para o shell de login configurado entrar em vigor",
		})
	}
	if summary.taskRan {
		next = append(next, []string{
			"Mensagens das tarefas", "revisar",
			"confira acima se algum instalador pediu reinício, login ou autenticação",
		})
	}
	if len(next) == 0 {
		next = append(next, []string{
			"Estado declarativo", "convergido", "nenhuma ação manual foi identificada pelo estado",
		})
	}
	output.WriteByte('\n')
	output.WriteString(ui.RenderTable([]string{"Próximo passo", "Situação", "Orientação"}, next))
	return output.String()
}
