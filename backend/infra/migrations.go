package infra

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

const (
	defaultMigrationTable = "schema_migrations"
	migrationAdvisoryLock = int64(749201120)
)

//go:embed migrations/*.sql
var embeddedMigrations embed.FS

type MigrationOptions struct {
	Table string
}

func RunMigrations(ctx context.Context, db *sql.DB, opts MigrationOptions) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}
	table := strings.TrimSpace(opts.Table)
	if table == "" {
		table = defaultMigrationTable
	}
	if !isSafeMigrationTableName(table) {
		return fmt.Errorf("invalid migration table name %q", table)
	}

	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("open migration connection: %w", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, `SELECT pg_advisory_lock($1)`, migrationAdvisoryLock); err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}
	defer conn.ExecContext(context.Background(), `SELECT pg_advisory_unlock($1)`, migrationAdvisoryLock)

	if _, err := conn.ExecContext(ctx, fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (version TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`, table)); err != nil {
		return fmt.Errorf("ensure migration table: %w", err)
	}

	files, err := listEmbeddedMigrationFiles()
	if err != nil {
		return err
	}
	for _, file := range files {
		version := filepath.Base(file)
		applied, err := migrationApplied(ctx, conn, table, version)
		if err != nil {
			return fmt.Errorf("check migration %s: %w", version, err)
		}
		if applied {
			continue
		}
		sqlBytes, err := embeddedMigrations.ReadFile(file)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", version, err)
		}
		if err := applyMigration(ctx, conn, table, version, string(sqlBytes)); err != nil {
			return err
		}
	}
	return nil
}

func listEmbeddedMigrationFiles() ([]string, error) {
	entries, err := fs.ReadDir(embeddedMigrations, "migrations")
	if err != nil {
		return nil, fmt.Errorf("read embedded migrations: %w", err)
	}
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".sql") {
			continue
		}
		files = append(files, filepath.Join("migrations", name))
	}
	sort.Strings(files)
	return files, nil
}

func migrationApplied(ctx context.Context, conn *sql.Conn, table, version string) (bool, error) {
	var applied int
	err := conn.QueryRowContext(ctx, fmt.Sprintf(`SELECT 1 FROM %s WHERE version = $1 LIMIT 1`, table), version).Scan(&applied)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return applied == 1, nil
}

func applyMigration(ctx context.Context, conn *sql.Conn, table, version, statements string) error {
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration %s: %w", version, err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, statements); err != nil {
		return fmt.Errorf("apply migration %s: %w", version, err)
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf(`INSERT INTO %s (version) VALUES ($1)`, table), version); err != nil {
		return fmt.Errorf("record migration %s: %w", version, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %s: %w", version, err)
	}
	return nil
}

func isSafeMigrationTableName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			continue
		}
		return false
	}
	return true
}
