// Package postgres provides a migration runner that applies all .up.sql files
// from a directory against the database on startup. Applied files are tracked in
// schema_migrations (shared with deploy/scripts/migrate.sh) so each file runs at
// most once; all migrations also use IF NOT EXISTS and are safe to re-run.
package postgres

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
)

// RunMigrations reads .up.sql files from dir (default "migrations"), sorts them
// by name, and applies any not yet recorded in schema_migrations. Each file is
// split into individual statements and executed sequentially.
func RunMigrations(c *Client, dir string) error {
	ctx := context.Background()
	if dir == "" {
		dir = "migrations"
	}
	slog.Info("running migrations", "dir", dir)

	if err := ensureMigrationsTable(ctx, c); err != nil {
		return err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read migrations dir %s: %w", dir, err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".up.sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	applied := 0
	for _, name := range files {
		done, err := migrationApplied(ctx, c, name)
		if err != nil {
			return err
		}
		if done {
			continue
		}

		path := filepath.Join(dir, name)
		sqlBytes, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}

		stmts := splitSQLStatements(string(sqlBytes))
		slog.Info("applying migration", "file", name, "statements", len(stmts))
		start := time.Now()
		for i, stmt := range stmts {
			if _, err := c.pool.Exec(ctx, stmt); err != nil {
				return fmt.Errorf("migration %s statement %d: %w\n%s", name, i+1, err, stmt)
			}
		}
		if err := markMigrationApplied(ctx, c, name); err != nil {
			return err
		}
		applied++
		slog.Info("migration applied", "file", name, "elapsed", time.Since(start))
	}

	slog.Info("migrations complete", "files", len(files), "applied", applied)
	return nil
}

func ensureMigrationsTable(ctx context.Context, c *Client) error {
	_, err := c.pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
    name       TEXT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
)`)
	if err != nil {
		return fmt.Errorf("ensure schema_migrations: %w", err)
	}
	return nil
}

func migrationApplied(ctx context.Context, c *Client, name string) (bool, error) {
	var exists bool
	err := c.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE name = $1)`, name).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check migration %s: %w", name, err)
	}
	return exists, nil
}

func markMigrationApplied(ctx context.Context, c *Client, name string) error {
	_, err := c.pool.Exec(ctx, `INSERT INTO schema_migrations (name) VALUES ($1) ON CONFLICT DO NOTHING`, name)
	if err != nil {
		return fmt.Errorf("mark migration %s: %w", name, err)
	}
	return nil
}

// splitSQLStatements splits raw SQL into individual statements on top-level
// semicolons. It is aware of:
//   - dollar-quoted blocks ($$...$$ and $tag$...$tag$), anywhere on a line;
//   - single-quoted string literals (with '' escapes);
//   - line comments (-- to end of line) and block comments (/* ... */).
//
// Semicolons inside any of the above are not treated as terminators.
func splitSQLStatements(sql string) []string {
	var stmts []string
	var cur strings.Builder
	runes := []rune(sql)
	n := len(runes)

	flush := func() {
		if s := strings.TrimSpace(cur.String()); s != "" {
			stmts = append(stmts, s)
		}
		cur.Reset()
	}

	for i := 0; i < n; {
		r := runes[i]

		// Line comment: -- ... end of line
		if r == '-' && i+1 < n && runes[i+1] == '-' {
			for i < n && runes[i] != '\n' {
				cur.WriteRune(runes[i])
				i++
			}
			continue
		}

		// Block comment: /* ... */
		if r == '/' && i+1 < n && runes[i+1] == '*' {
			cur.WriteRune(runes[i])
			cur.WriteRune(runes[i+1])
			i += 2
			for i < n && !(runes[i] == '*' && i+1 < n && runes[i+1] == '/') {
				cur.WriteRune(runes[i])
				i++
			}
			if i < n {
				cur.WriteRune(runes[i]) // *
				i++
			}
			if i < n {
				cur.WriteRune(runes[i]) // /
				i++
			}
			continue
		}

		// Single-quoted string literal (handles '' escape).
		if r == '\'' {
			cur.WriteRune(r)
			i++
			for i < n {
				cur.WriteRune(runes[i])
				if runes[i] == '\'' {
					if i+1 < n && runes[i+1] == '\'' {
						cur.WriteRune(runes[i+1])
						i += 2
						continue
					}
					i++
					break
				}
				i++
			}
			continue
		}

		// Dollar-quoted block: $tag$ ... $tag$ (tag may be empty → $$).
		if r == '$' {
			if tag, ok := matchDollarTag(runes, i); ok {
				cur.WriteString(tag)
				i += len([]rune(tag))
				for i < n {
					if runes[i] == '$' {
						if t2, ok2 := matchDollarTag(runes, i); ok2 && t2 == tag {
							cur.WriteString(t2)
							i += len([]rune(t2))
							break
						}
					}
					cur.WriteRune(runes[i])
					i++
				}
				continue
			}
		}

		// Top-level statement terminator.
		if r == ';' {
			cur.WriteRune(r)
			i++
			flush()
			continue
		}

		cur.WriteRune(r)
		i++
	}

	flush()
	return stmts
}

// matchDollarTag reports whether runes[i:] begins with a dollar-quote tag of the
// form $...$ where the inner label is empty or a valid identifier (not starting
// with a digit). It returns the full tag including both '$' delimiters.
func matchDollarTag(runes []rune, i int) (string, bool) {
	if i >= len(runes) || runes[i] != '$' {
		return "", false
	}
	j := i + 1
	for j < len(runes) && isIdentRune(runes[j]) {
		j++
	}
	if j < len(runes) && runes[j] == '$' {
		// A non-empty label must not start with a digit (avoids matching $1$).
		if j > i+1 && unicode.IsDigit(runes[i+1]) {
			return "", false
		}
		return string(runes[i : j+1]), true
	}
	return "", false
}

func isIdentRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}
