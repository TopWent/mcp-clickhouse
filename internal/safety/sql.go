// Package safety enforces read-only SQL access for tools that accept
// arbitrary input from an LLM.
package safety

import (
	"fmt"
	"strings"
)

var allowedLeadingKeywords = []string{"SELECT", "EXPLAIN"}

// EnforceReadOnly rejects queries that are not SELECT or EXPLAIN, or
// that contain unquoted statement separators.
func EnforceReadOnly(query string) error {
	stripped, err := stripLeadingCommentsWS(query)
	if err != nil {
		return err
	}
	if stripped == "" {
		return fmt.Errorf("empty query")
	}

	keyword := firstKeyword(stripped)
	if !isAllowedKeyword(keyword) {
		return fmt.Errorf("only SELECT or EXPLAIN allowed (got %s)", keyword)
	}

	if hasUnquotedSemicolon(stripped) {
		return fmt.Errorf("multi-statement queries not allowed")
	}

	return nil
}

func isAllowedKeyword(kw string) bool {
	upper := strings.ToUpper(kw)
	for _, allowed := range allowedLeadingKeywords {
		if upper == allowed {
			return true
		}
	}
	return false
}

func firstKeyword(s string) string {
	end := 0
	for end < len(s) {
		c := s[end]
		if !isLetter(c) {
			break
		}
		end++
	}
	return s[:end]
}

func isLetter(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func stripLeadingCommentsWS(s string) (string, error) {
	for {
		s = strings.TrimLeft(s, " \t\r\n")
		if strings.HasPrefix(s, "--") {
			i := strings.IndexByte(s, '\n')
			if i < 0 {
				return "", nil
			}
			s = s[i+1:]
			continue
		}
		if strings.HasPrefix(s, "/*") {
			end := strings.Index(s, "*/")
			if end < 0 {
				return "", fmt.Errorf("unterminated block comment")
			}
			s = s[end+2:]
			continue
		}
		break
	}
	return s, nil
}

func hasUnquotedSemicolon(s string) bool {
	var (
		inSingle, inDouble, inBacktick bool
		inLine, inBlock                bool
	)
	n := len(s)
	for i := 0; i < n; i++ {
		c := s[i]
		switch {
		case inLine:
			if c == '\n' {
				inLine = false
			}
		case inBlock:
			if c == '*' && i+1 < n && s[i+1] == '/' {
				inBlock = false
				i++
			}
		case inSingle:
			if c == '\\' && i+1 < n {
				i++
				continue
			}
			if c == '\'' {
				if i+1 < n && s[i+1] == '\'' {
					i++
					continue
				}
				inSingle = false
			}
		case inDouble:
			if c == '"' {
				inDouble = false
			}
		case inBacktick:
			if c == '`' {
				inBacktick = false
			}
		default:
			switch c {
			case '\'':
				inSingle = true
			case '"':
				inDouble = true
			case '`':
				inBacktick = true
			case '-':
				if i+1 < n && s[i+1] == '-' {
					inLine = true
					i++
				}
			case '/':
				if i+1 < n && s[i+1] == '*' {
					inBlock = true
					i++
				}
			case ';':
				if strings.TrimSpace(s[i+1:]) != "" {
					return true
				}
			}
		}
	}
	return false
}
