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

// AddTaskRunReference appends a sequential mise task reference to
// [tasks.bootstrap].run without reformatting unrelated TOML. Sequential run
// entries are intentional for personal installers: two package-manager tasks
// must not contend for the same system lock.
func AddTaskRunReference(document []byte, task string) ([]byte, bool, error) {
	var decoded map[string]any
	if err := toml.Unmarshal(document, &decoded); err != nil {
		return nil, false, err
	}
	quotedTask, err := TOMLString(task)
	if err != nil {
		return nil, false, err
	}
	literal := "{ task = " + quotedTask + " }"

	run, exists, err := tableEntryValue(decoded, []string{"tasks", "bootstrap"}, "run")
	if err != nil {
		return nil, false, err
	}
	if !exists {
		entry := "run = [\n  " + literal + ",\n]\n"
		result := insertTableEntry(document, "[tasks.bootstrap]", entry)
		if err := toml.Unmarshal(result, &decoded); err != nil {
			return nil, false, fmt.Errorf("a alteração produziria TOML inválido: %w", err)
		}
		return result, false, nil
	}

	entries, ok := run.([]any)
	if !ok {
		return nil, false, fmt.Errorf("tasks.bootstrap.run precisa ser uma lista para receber %s", task)
	}
	for _, entry := range entries {
		record, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		if existing, ok := record["task"].(string); ok && existing == task {
			return append([]byte(nil), document...), true, nil
		}
	}

	result, err := appendArrayElement(document, "[tasks.bootstrap]", "run", literal)
	if err != nil {
		return nil, false, err
	}
	if err := toml.Unmarshal(result, &decoded); err != nil {
		return nil, false, fmt.Errorf("a alteração produziria TOML inválido: %w", err)
	}
	return result, false, nil
}

func tableEntryValue(document map[string]any, table []string, key string) (any, bool, error) {
	var current any = document
	for _, segment := range table {
		mapping, ok := current.(map[string]any)
		if !ok {
			return nil, false, fmt.Errorf("a tabela %s é incompatível", strings.Join(table, "."))
		}
		current, ok = mapping[segment]
		if !ok {
			return nil, false, nil
		}
	}
	mapping, ok := current.(map[string]any)
	if !ok {
		return nil, false, fmt.Errorf("a tabela %s é incompatível", strings.Join(table, "."))
	}
	value, exists := mapping[key]
	return value, exists, nil
}

func appendArrayElement(document []byte, header, key, literal string) ([]byte, error) {
	sectionStart, sectionEnd, ok := tableSectionBounds(document, header)
	if !ok {
		return nil, fmt.Errorf("a seção %s não foi encontrada", header)
	}
	open, close, ok := arrayAssignmentBounds(document, sectionStart, sectionEnd, key)
	if !ok {
		return nil, fmt.Errorf("%s.%s usa um formato que o assistente ainda não consegue preservar", header, key)
	}

	inside := document[open+1 : close]
	assignmentLineStart := bytes.LastIndex(document[:open], []byte("\n")) + 1
	assignmentIndent := leadingIndent(document[assignmentLineStart:open])
	if len(bytes.TrimSpace(inside)) == 0 {
		replacement := "\n" + assignmentIndent + "  " + literal + ",\n" + assignmentIndent
		return replaceBytes(document, open+1, close, []byte(replacement)), nil
	}

	closeLineStart := bytes.LastIndex(document[:close], []byte("\n")) + 1
	if closeLineStart > open && len(bytes.TrimSpace(document[closeLineStart:close])) == 0 {
		indent := string(document[closeLineStart:close]) + "  "
		result := append([]byte(nil), document...)
		last := lastArrayValueByte(document, open+1, closeLineStart)
		if last < 0 {
			return nil, fmt.Errorf("não foi possível localizar o último item de %s.%s", header, key)
		}
		if result[last] != ',' {
			result = insertBytes(result, last+1, []byte(","))
			closeLineStart++
		}
		return insertBytes(result, closeLineStart, []byte(indent+literal+",\n")), nil
	}

	insertAt := close
	for insertAt > open+1 && (document[insertAt-1] == ' ' || document[insertAt-1] == '\t') {
		insertAt--
	}
	last := lastNonSpaceBefore(document, insertAt)
	separator := ", "
	if last >= 0 && document[last] == ',' {
		separator = " "
	}
	return insertBytes(document, insertAt, []byte(separator+literal)), nil
}

func tableSectionBounds(document []byte, header string) (int, int, bool) {
	lines := bytes.SplitAfter(document, []byte("\n"))
	start := -1
	offset := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(string(line))
		if start < 0 {
			if trimmed == header {
				start = offset + len(line)
			}
		} else if isTableHeader(trimmed) {
			return start, offset, true
		}
		offset += len(line)
	}
	if start >= 0 {
		return start, len(document), true
	}
	return 0, 0, false
}

func arrayAssignmentBounds(document []byte, start, end int, key string) (int, int, bool) {
	offset := start
	for offset < end {
		lineEnd := bytes.IndexByte(document[offset:end], '\n')
		if lineEnd < 0 {
			lineEnd = end - offset
		} else {
			lineEnd++
		}
		line := document[offset : offset+lineEnd]
		trimmed := bytes.TrimLeft(line, " \t")
		keyLength := assignmentKeyLength(trimmed, key)
		if keyLength > 0 {
			rest := bytes.TrimLeft(trimmed[keyLength:], " \t")
			if len(rest) > 0 && rest[0] == '=' {
				valueStart := offset + len(line) - len(rest) + 1
				open, close, ok := findArrayBounds(document, valueStart, end)
				return open, close, ok
			}
		}
		offset += lineEnd
	}
	return 0, 0, false
}

func assignmentKeyLength(line []byte, key string) int {
	if bytes.HasPrefix(line, []byte(key)) {
		return len(key)
	}
	for _, quote := range []byte{'"', '\''} {
		quoted := append([]byte{quote}, []byte(key)...)
		quoted = append(quoted, quote)
		if bytes.HasPrefix(line, quoted) {
			return len(quoted)
		}
	}
	return 0
}

func findArrayBounds(document []byte, start, end int) (int, int, bool) {
	const (
		normal = iota
		basicString
		literalString
		multilineBasic
		multilineLiteral
		comment
	)
	mode := normal
	depth := 0
	open := -1
	for index := start; index < end; index++ {
		character := document[index]
		switch mode {
		case comment:
			if character == '\n' {
				mode = normal
			}
		case basicString:
			if character == '\\' {
				index++
			} else if character == '"' {
				mode = normal
			}
		case literalString:
			if character == '\'' {
				mode = normal
			}
		case multilineBasic:
			if character == '"' && index+2 < end && string(document[index:index+3]) == `"""` {
				mode = normal
				index += 2
			}
		case multilineLiteral:
			if character == '\'' && index+2 < end && string(document[index:index+3]) == "'''" {
				mode = normal
				index += 2
			}
		default:
			switch character {
			case '#':
				mode = comment
			case '"':
				if index+2 < end && string(document[index:index+3]) == `"""` {
					mode = multilineBasic
					index += 2
				} else {
					mode = basicString
				}
			case '\'':
				if index+2 < end && string(document[index:index+3]) == "'''" {
					mode = multilineLiteral
					index += 2
				} else {
					mode = literalString
				}
			case '[':
				if open < 0 {
					open = index
				}
				depth++
			case ']':
				if open >= 0 {
					depth--
					if depth == 0 {
						return open, index, true
					}
				}
			}
		}
	}
	return 0, 0, false
}

func leadingIndent(line []byte) string {
	length := 0
	for length < len(line) && (line[length] == ' ' || line[length] == '\t') {
		length++
	}
	return string(line[:length])
}

func lastNonSpaceBefore(document []byte, end int) int {
	for index := end - 1; index >= 0; index-- {
		switch document[index] {
		case ' ', '\t', '\r', '\n':
			continue
		default:
			return index
		}
	}
	return -1
}

func lastArrayValueByte(document []byte, start, end int) int {
	last := -1
	lineStart := start
	for lineStart < end {
		lineLength := bytes.IndexByte(document[lineStart:end], '\n')
		lineEnd := end
		if lineLength >= 0 {
			lineEnd = lineStart + lineLength
		}
		contentEnd := lineEnd
		if comment := tomlCommentStart(document[lineStart:lineEnd]); comment >= 0 {
			contentEnd = lineStart + comment
		}
		for index := contentEnd - 1; index >= lineStart; index-- {
			if document[index] != ' ' && document[index] != '\t' && document[index] != '\r' {
				last = index
				break
			}
		}
		lineStart = lineEnd + 1
	}
	return last
}

func tomlCommentStart(line []byte) int {
	mode := byte(0)
	escaped := false
	for index, character := range line {
		switch mode {
		case '"':
			if escaped {
				escaped = false
				continue
			}
			if character == '\\' {
				escaped = true
			} else if character == '"' {
				mode = 0
			}
		case '\'':
			if character == '\'' {
				mode = 0
			}
		default:
			if character == '"' || character == '\'' {
				mode = character
			} else if character == '#' {
				return index
			}
		}
	}
	return -1
}

func insertBytes(document []byte, at int, value []byte) []byte {
	result := make([]byte, 0, len(document)+len(value))
	result = append(result, document[:at]...)
	result = append(result, value...)
	return append(result, document[at:]...)
}

func replaceBytes(document []byte, start, end int, value []byte) []byte {
	result := make([]byte, 0, len(document)-(end-start)+len(value))
	result = append(result, document[:start]...)
	result = append(result, value...)
	return append(result, document[end:]...)
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
