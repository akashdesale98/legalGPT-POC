package db

import (
	"context"
	_ "embed"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/001_schema.sql
var migration001 string

//go:embed migrations/002_rls.sql
var migration002 string

// migrationLockID is the advisory lock ID used to prevent concurrent migration runs.
const migrationLockID = 42_424_242

// RunMigrations applies all migrations in order. Every migration is idempotent.
// Uses a PostgreSQL advisory lock to prevent concurrent migration runs across replicas.
func RunMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	// Acquire advisory lock (session-level, released when connection returns to pool)
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("migrations: acquire conn: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", migrationLockID); err != nil {
		return fmt.Errorf("migrations: advisory lock: %w", err)
	}
	defer func() {
		if _, unlockErr := conn.Exec(ctx, "SELECT pg_advisory_unlock($1)", migrationLockID); unlockErr != nil {
			log.Printf("WARN: migrations: advisory unlock: %v", unlockErr)
		}
	}()

	migrations := []struct {
		name string
		sql  string
	}{
		{"001_schema", migration001},
		{"002_rls", migration002},
	}

	for _, m := range migrations {
		log.Printf("DB: running migration %s", m.name)
		if _, err := conn.Exec(ctx, m.sql); err != nil {
			return fmt.Errorf("migration %s: %w", m.name, err)
		}
		log.Printf("DB: migration %s ok", m.name)
	}
	return nil
}
