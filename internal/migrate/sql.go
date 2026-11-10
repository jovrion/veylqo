package migrate

import (
	"errors"
	"strings"
)

// versionName is the filename rule. Four digits keep lexical order equal
// to numeric order through 9999. The rest is a lowercase slug.
func versionName(name string) bool {
	if len(name) < len("0000_a.sql") || !strings.HasSuffix(name, ".sql") {
		return false
	}
	base := strings.TrimSuffix(name, ".sql")
	prefix, slug, ok := strings.Cut(base, "_")
	if !ok || len(prefix) != 4 || slug == "" {
		return false
	}
	for i := 0; i < len(prefix); i++ {
		if prefix[i] < '0' || prefix[i] > '9' {
			return false
		}
	}
	for i := 0; i < len(slug); i++ {
		c := slug[i]
		if c != '_' && (c < 'a' || c > 'z') && (c < '0' || c > '9') {
			return false
		}
	}
	return true
}

func rejectTxnControl(stmt string) error {
	word := firstWord(stmt)
	switch strings.ToUpper(word) {
	case "BEGIN", "COMMIT", "ROLLBACK", "END":
		return errors.New("migration must not manage transactions")
	default:
		return nil
	}
}

func firstWord(stmt string) string {
	fields := strings.Fields(stmt)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// splitSQL splits a migration file on semicolons that are not inside
// quotes, comments, or dollar quotes. Migrations must not use tagged
// dollar quotes ($tag$). An unclosed quote is an error so a partial
// file is not applied.
func splitSQL(sql string) ([]string, error) {
	var b strings.Builder
	var out []string
	inSingle := false
	inDouble := false
	inLine := false
	inBlock := false
	inDollar := false
	for i := 0; i < len(sql); i++ {
		c := sql[i]
		var next byte
		if i+1 < len(sql) {
			next = sql[i+1]
		}
		switch {
		case inLine:
			if c == '\n' {
				inLine = false
			}
		case inBlock:
			if c == '*' && next == '/' {
				inBlock = false
				i++
			}
		case inDollar:
			if c == '$' && next == '$' {
				inDollar = false
				b.WriteString("$$")
				i++
				continue
			}
			b.WriteByte(c)
		case inSingle:
			b.WriteByte(c)
			if c == '\'' && next == '\'' {
				b.WriteByte(next)
				i++
				continue
			}
			if c == '\'' {
				inSingle = false
			}
		case inDouble:
			b.WriteByte(c)
			if c == '"' {
				inDouble = false
			}
		case c == '-' && next == '-':
			inLine = true
			i++
		case c == '/' && next == '*':
			inBlock = true
			i++
		case c == '\'':
			inSingle = true
			b.WriteByte(c)
		case c == '"':
			inDouble = true
			b.WriteByte(c)
		case c == '$' && next == '$':
			inDollar = true
			b.WriteString("$$")
			i++
		case c == ';':
			if stmt := strings.TrimSpace(b.String()); stmt != "" {
				out = append(out, stmt)
			}
			b.Reset()
		default:
			b.WriteByte(c)
		}
	}
	if inSingle || inDouble || inBlock || inDollar {
		return nil, errors.New("unclosed sql")
	}
	if stmt := strings.TrimSpace(b.String()); stmt != "" {
		out = append(out, stmt)
	}
	return out, nil
}
