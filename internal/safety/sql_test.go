package safety

import (
	"strings"
	"testing"
)

func TestEnforceReadOnly_AllowsSelect(t *testing.T) {
	cases := []string{
		"SELECT 1",
		"select 1",
		"Select 1",
		"  SELECT 1",
		"\tSELECT 1\n",
		"SELECT * FROM users",
		"SELECT * FROM users;",
		"SELECT * FROM users; ",
		"SELECT * FROM users;\n",
		"SELECT 'hello;world' FROM t",
		"SELECT \"col;name\" FROM t",
		"SELECT `id;here` FROM t",
		"SELECT 'it''s a test' FROM t",
		"SELECT '\\'quoted\\'' FROM t",
		"SELECT 1 /* mid; query comment */",
		"SELECT 1 -- trailing comment",
		"/* leading block */ SELECT 1",
		"-- leading line\nSELECT 1",
		"EXPLAIN SELECT * FROM users",
		"explain SELECT 1",
	}
	for _, q := range cases {
		t.Run(q, func(t *testing.T) {
			if err := EnforceReadOnly(q); err != nil {
				t.Errorf("EnforceReadOnly(%q) = %v, want nil", q, err)
			}
		})
	}
}

func TestEnforceReadOnly_RejectsWrites(t *testing.T) {
	cases := []struct {
		query   string
		wantErr string
	}{
		{"INSERT INTO users VALUES (1)", "only SELECT or EXPLAIN"},
		{"UPDATE users SET name='x'", "only SELECT or EXPLAIN"},
		{"DELETE FROM users", "only SELECT or EXPLAIN"},
		{"DROP TABLE users", "only SELECT or EXPLAIN"},
		{"TRUNCATE TABLE users", "only SELECT or EXPLAIN"},
		{"CREATE TABLE x (id INT)", "only SELECT or EXPLAIN"},
		{"ALTER TABLE x ADD COLUMN y INT", "only SELECT or EXPLAIN"},
		{"ATTACH TABLE x", "only SELECT or EXPLAIN"},
		{"DETACH TABLE x", "only SELECT or EXPLAIN"},
		{"OPTIMIZE TABLE x", "only SELECT or EXPLAIN"},
		{"RENAME TABLE x TO y", "only SELECT or EXPLAIN"},
		{"GRANT SELECT ON x TO user", "only SELECT or EXPLAIN"},
		{"REVOKE SELECT ON x FROM user", "only SELECT or EXPLAIN"},
		{"SET allow_experimental=1", "only SELECT or EXPLAIN"},
		{"SYSTEM RELOAD CONFIG", "only SELECT or EXPLAIN"},
		{"WITH x AS (SELECT 1) SELECT * FROM x", "only SELECT or EXPLAIN"},
		{"SHOW DATABASES", "only SELECT or EXPLAIN"},
		{"DESCRIBE TABLE x", "only SELECT or EXPLAIN"},
		{"DESC users", "only SELECT or EXPLAIN"},
		{"USE other_db", "only SELECT or EXPLAIN"},
		{"(SELECT 1)", "only SELECT or EXPLAIN"},
	}
	for _, tc := range cases {
		t.Run(tc.query, func(t *testing.T) {
			err := EnforceReadOnly(tc.query)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("err = %q, want to contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestEnforceReadOnly_RejectsMultiStatement(t *testing.T) {
	cases := []string{
		"SELECT 1; DROP TABLE users",
		"SELECT 1;DROP TABLE users",
		"SELECT 1; SELECT 2",
		"SELECT 1; -- still injection\nDROP TABLE users",
		"SELECT * FROM users WHERE id=1; INSERT INTO logs VALUES (1)",
	}
	for _, q := range cases {
		t.Run(q, func(t *testing.T) {
			err := EnforceReadOnly(q)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !strings.Contains(err.Error(), "multi-statement") {
				t.Errorf("err = %q, want to contain 'multi-statement'", err.Error())
			}
		})
	}
}

func TestEnforceReadOnly_EdgeCases(t *testing.T) {
	cases := []struct {
		name    string
		query   string
		wantErr string
	}{
		{"empty", "", "empty query"},
		{"whitespace only", "   \t\n", "empty query"},
		{"line comment only", "-- just a comment", "empty query"},
		{"block comment only", "/* just a comment */", "empty query"},
		{"unterminated block comment", "/* never closes", "unterminated"},
		{"unterminated nested looking comment", "/* /* ", "unterminated"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := EnforceReadOnly(tc.query)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("err = %q, want to contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestEnforceReadOnly_AttackPatterns(t *testing.T) {
	cases := []struct {
		name  string
		query string
	}{
		{"comment-wrapped DROP after leading comment", "/* harmless */ DROP TABLE y"},
		{"select then drop", "SELECT 1; DROP DATABASE prod"},
		{"select then insert", "SELECT * FROM users WHERE id=1; INSERT INTO logs VALUES(1)"},
		{"trailing comment hiding DROP", "SELECT 1; /* hidden */ DROP TABLE x"},
		{"line comment trying to hide", "SELECT 1; -- hidden\nDROP TABLE x"},
		{"select followed by system command", "SELECT 1; SYSTEM RELOAD CONFIG"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := EnforceReadOnly(tc.query); err == nil {
				t.Errorf("EnforceReadOnly(%q) returned nil, want rejection", tc.query)
			}
		})
	}
}

func TestEnforceReadOnly_SemicolonInsideStringIsAllowed(t *testing.T) {
	cases := []string{
		`SELECT 'a;b;c' FROM t`,
		`SELECT "col;name" FROM t`,
		"SELECT `weird;ident` FROM t",
		`SELECT 1 /* a;b;c */`,
		`SELECT 1 -- comment;with;semis`,
		`SELECT 'a''b;c' FROM t`,
	}
	for _, q := range cases {
		t.Run(q, func(t *testing.T) {
			if err := EnforceReadOnly(q); err != nil {
				t.Errorf("EnforceReadOnly(%q) = %v, want nil", q, err)
			}
		})
	}
}
