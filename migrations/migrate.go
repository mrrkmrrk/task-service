package migrations

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

//go:embed *.sql
var files embed.FS

// Apply serializes startup migrations on one connection. DDL is idempotent because
// MySQL commits DDL implicitly; a failed migration can be retried safely.
func Apply(ctx context.Context, db *sql.DB) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("migration connection: %w", err)
	}
	defer conn.Close()
	var locked int
	if err := conn.QueryRowContext(ctx, "SELECT GET_LOCK(CONCAT(DATABASE(), ':migrations'), 30)").Scan(&locked); err != nil {
		return fmt.Errorf("migration lock: %w", err)
	}
	if locked != 1 {
		return fmt.Errorf("migration lock timed out")
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = conn.ExecContext(cleanup, "DO RELEASE_LOCK(CONCAT(DATABASE(), ':migrations'))")
	}()
	if _, err := conn.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (name VARCHAR(255) PRIMARY KEY, checksum CHAR(64) NOT NULL, applied_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6))`); err != nil {
		return fmt.Errorf("migration table: %w", err)
	}
	entries, err := files.ReadDir(".")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		body, err := files.ReadFile(entry.Name())
		if err != nil {
			return err
		}
		sum := sha256.Sum256(body)
		checksum := hex.EncodeToString(sum[:])
		var previous string
		err = conn.QueryRowContext(ctx, "SELECT checksum FROM schema_migrations WHERE name = ?", entry.Name()).Scan(&previous)
		if err == nil {
			if previous != checksum {
				return fmt.Errorf("migration %s checksum mismatch", entry.Name())
			}
			continue
		}
		if err != sql.ErrNoRows {
			return fmt.Errorf("migration lookup: %w", err)
		}
		for _, statement := range strings.Split(string(body), ";") {
			if strings.TrimSpace(statement) == "" {
				continue
			}
			if _, err := conn.ExecContext(ctx, statement); err != nil {
				return fmt.Errorf("migration %s: %w", entry.Name(), err)
			}
		}
		if _, err := conn.ExecContext(ctx, "INSERT INTO schema_migrations (name, checksum) VALUES (?, ?)", entry.Name(), checksum); err != nil {
			return fmt.Errorf("record migration: %w", err)
		}
	}
	return nil
}
