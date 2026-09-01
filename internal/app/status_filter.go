package app

import (
	"fmt"
	"sort"
	"strings"
)

func filterStatusRows(rows []statusRow, only, states []string) []statusRow {
	filtered := make([]statusRow, 0, len(rows))
	for _, row := range rows {
		if len(only) > 0 && !containsString(only, row.part) {
			continue
		}
		if len(states) > 0 && !statusRowMatchesAnyState(row, states) {
			continue
		}
		filtered = append(filtered, row)
	}
	return filtered
}

func statusRowMatchesAnyState(row statusRow, states []string) bool {
	for _, state := range states {
		switch state {
		case "ready":
			if statusRowStateGroup(row) == "ready" {
				return true
			}
		case "pending":
			if statusRowStateGroup(row) == "pending" {
				return true
			}
		case "missing":
			if row.stateKey == "missing" || row.stateKey == "source_missing" || row.state == "ausente" || row.state == "fonte ausente" {
				return true
			}
		case "different":
			if row.stateKey == "differs" || row.state == "diferente" {
				return true
			}
		case "unknown":
			if statusRowStateGroup(row) == "unknown" {
				return true
			}
		}
	}
	return false
}

func statusRowStateGroup(row statusRow) string {
	switch row.stateKey {
	case "applied", "installed", "current", "set", "running", "enabled", "available":
		return "ready"
	case "missing", "source_missing", "differs", "pending", "stopped", "disabled", "not_executable":
		return "pending"
	}
	if isConvergedState(row.state) || row.state == "disponível" {
		return "ready"
	}
	switch row.state {
	case "ausente", "fonte ausente", "diferente", "pendente", "parado", "desabilitado", "não executável":
		return "pending"
	default:
		return "unknown"
	}
}

func validateStatusParts(parts []string) error {
	known := map[string]bool{
		"packages": true, "repos": true, "dotfiles": true,
		"tools": true, "task": true, "user": true,
	}
	return validateStatusFilterValues(
		parts, known, "categoria", "packages, repos, dotfiles, tools, task ou user",
	)
}

func validateStatusStates(states []string) error {
	known := map[string]bool{
		"ready": true, "pending": true, "missing": true, "different": true, "unknown": true,
	}
	return validateStatusFilterValues(
		states, known, "situação", "ready, pending, missing, different ou unknown",
	)
}

func validateStatusFilterValues(values []string, known map[string]bool, kind, guidance string) error {
	var invalid []string
	for _, value := range values {
		if !known[value] {
			invalid = append(invalid, value)
		}
	}
	if len(invalid) == 0 {
		return nil
	}
	sort.Strings(invalid)
	return fmt.Errorf("%s desconhecida: %s (use %s)", kind, strings.Join(invalid, ", "), guidance)
}
