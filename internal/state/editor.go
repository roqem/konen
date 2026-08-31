package state

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// AddTableEntry adds one quoted key with a caller-supplied TOML literal while
// preserving the rest of the document byte-for-byte. It intentionally does not
// update existing keys: guided "add" commands must never reinterpret a more
// expressive declaration the user already wrote.
func AddTableEntry(document []byte, table []string, key, literal string) ([]byte, bool, error) {
	if len(table) == 0 {
		return nil, false, fmt.Errorf("a tabela TOML não pode ser vazia")
	}
	var decoded map[string]any
	if err := toml.Unmarshal(document, &decoded); err != nil {
		return nil, false, err
	}
	if tableEntryExists(decoded, table, key) {
		return append([]byte(nil), document...), true, nil
	}

	entry := strconv.Quote(key) + " = " + literal + "\n"
	if err := validateEntryLiteral(entry); err != nil {
		return nil, false, err
	}
	header := "[" + strings.Join(table, ".") + "]"
	result := insertTableEntry(document, header, entry)
	if err := toml.Unmarshal(result, &decoded); err != nil {
		return nil, false, fmt.Errorf("a alteração produziria TOML inválido: %w", err)
	}
	return result, false, nil
}

func TOMLString(value string) (string, error) {
	quoted := strconv.Quote(value)
	if err := validateEntryLiteral("value = " + quoted + "\n"); err != nil {
		return "", err
	}
	return quoted, nil
}

func tableEntryExists(document map[string]any, table []string, key string) bool {
	var current any = document
	for _, segment := range table {
		mapping, ok := current.(map[string]any)
		if !ok {
			return false
		}
		current, ok = mapping[segment]
		if !ok {
			return false
		}
	}
	mapping, ok := current.(map[string]any)
	if !ok {
		return false
	}
	_, exists := mapping[key]
	return exists
}

func validateEntryLiteral(entry string) error {
	var value map[string]any
	if err := toml.Unmarshal([]byte(entry), &value); err != nil {
		return fmt.Errorf("valor TOML inválido: %w", err)
	}
	return nil
}

func insertTableEntry(document []byte, header, entry string) []byte {
	lines := bytes.SplitAfter(document, []byte("\n"))
	section := -1
	insertAt := len(document)
	offset := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(string(line))
		if section < 0 {
			if trimmed == header {
				section = offset + len(line)
				insertAt = len(document)
			}
		} else if isTableHeader(trimmed) {
			insertAt = offset
			break
		}
		offset += len(line)
	}

	if section < 0 {
		result := append([]byte(nil), document...)
		if len(result) > 0 && result[len(result)-1] != '\n' {
			result = append(result, '\n')
		}
		if len(result) > 0 && !bytes.HasSuffix(result, []byte("\n\n")) {
			result = append(result, '\n')
		}
		result = append(result, header...)
		result = append(result, '\n')
		return append(result, entry...)
	}

	prefix := append([]byte(nil), document[:insertAt]...)
	if len(prefix) > 0 && prefix[len(prefix)-1] != '\n' {
		prefix = append(prefix, '\n')
	}
	result := append(prefix, entry...)
	return append(result, document[insertAt:]...)
}

func isTableHeader(line string) bool {
	return strings.HasPrefix(line, "[") && strings.Contains(line, "]")
}
