package config

import (
	"fmt"
	"strconv"
	"strings"
)

// parseTOML parses the small subset of TOML the config file needs: [tables],
// key = value pairs, comments, and values that are strings, integers,
// booleans, or (possibly multi-line) arrays of those. Anything else is an
// error, so a typo is reported rather than silently ignored.
func parseTOML(text string) (map[string]map[string]any, error) {
	out := map[string]map[string]any{"": {}}
	table := ""
	lines := strings.Split(text, "\n")
	for i := 0; i < len(lines); i++ {
		lineNo := i + 1
		line := strings.TrimSpace(stripComment(lines[i]))
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") {
			if !strings.HasSuffix(line, "]") || strings.HasPrefix(line, "[[") {
				return nil, fmt.Errorf("line %d: unsupported table header %q", lineNo, line)
			}
			table = strings.TrimSpace(line[1 : len(line)-1])
			if table == "" {
				return nil, fmt.Errorf("line %d: empty table name", lineNo)
			}
			if out[table] == nil {
				out[table] = map[string]any{}
			}
			continue
		}
		eq := strings.Index(line, "=")
		if eq < 0 {
			return nil, fmt.Errorf("line %d: expected key = value", lineNo)
		}
		key := strings.TrimSpace(line[:eq])
		raw := strings.TrimSpace(line[eq+1:])
		if key == "" {
			return nil, fmt.Errorf("line %d: missing key", lineNo)
		}
		// Multi-line arrays: keep consuming lines until brackets balance.
		for strings.HasPrefix(raw, "[") && bracketDepth(raw) > 0 && i+1 < len(lines) {
			i++
			raw += " " + strings.TrimSpace(stripComment(lines[i]))
		}
		v, err := parseValue(raw)
		if err != nil {
			return nil, fmt.Errorf("line %d: %s: %w", lineNo, key, err)
		}
		out[table][key] = v
	}
	return out, nil
}

// stripComment removes a trailing # comment that is not inside a string.
func stripComment(s string) string {
	var quote byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case quote != 0:
			if c == '\\' && quote == '"' {
				i++
			} else if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '#':
			return s[:i]
		}
	}
	return s
}

func bracketDepth(s string) int {
	depth := 0
	var quote byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case quote != 0:
			if c == '\\' && quote == '"' {
				i++
			} else if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '[':
			depth++
		case c == ']':
			depth--
		}
	}
	return depth
}

func parseValue(raw string) (any, error) {
	switch {
	case raw == "":
		return nil, fmt.Errorf("missing value")
	case raw == "true":
		return true, nil
	case raw == "false":
		return false, nil
	case raw[0] == '"':
		if len(raw) < 2 || raw[len(raw)-1] != '"' {
			return nil, fmt.Errorf("unterminated string")
		}
		s, err := strconv.Unquote(raw)
		if err != nil {
			return nil, fmt.Errorf("bad string %s", raw)
		}
		return s, nil
	case raw[0] == '\'':
		if len(raw) < 2 || raw[len(raw)-1] != '\'' {
			return nil, fmt.Errorf("unterminated string")
		}
		return raw[1 : len(raw)-1], nil
	case raw[0] == '[':
		if raw[len(raw)-1] != ']' {
			return nil, fmt.Errorf("unterminated array")
		}
		return parseArray(raw[1 : len(raw)-1])
	default:
		n, err := strconv.ParseInt(strings.ReplaceAll(raw, "_", ""), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("unsupported value %s", raw)
		}
		return n, nil
	}
}

func parseArray(body string) ([]any, error) {
	var items []any
	var cur strings.Builder
	var quote byte
	flush := func() error {
		s := strings.TrimSpace(cur.String())
		cur.Reset()
		if s == "" {
			return nil
		}
		v, err := parseValue(s)
		if err != nil {
			return err
		}
		if _, nested := v.([]any); nested {
			return fmt.Errorf("nested arrays are not supported")
		}
		items = append(items, v)
		return nil
	}
	for i := 0; i < len(body); i++ {
		c := body[i]
		switch {
		case quote != 0:
			cur.WriteByte(c)
			if c == '\\' && quote == '"' && i+1 < len(body) {
				i++
				cur.WriteByte(body[i])
			} else if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
			cur.WriteByte(c)
		case c == ',':
			if err := flush(); err != nil {
				return nil, err
			}
		default:
			cur.WriteByte(c)
		}
	}
	if err := flush(); err != nil {
		return nil, err
	}
	return items, nil
}
