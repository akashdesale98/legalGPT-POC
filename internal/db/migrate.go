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

// RunMigrations applies all migrations in order. Every migration is idempotent.
func RunMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	migrations := []struct {
		name string
		sql  string
	}{
		{"001_schema", migration001},
		{"002_rls", migration002},
	}

	for _, m := range migrations {
		log.Printf("DB: running migration %s", m.name)
		if _, err := pool.Exec(ctx, m.sql); err != nil {
			return fmt.Errorf("migration %s: %w", m.name, err)
		}
		log.Printf("DB: migration %s ok", m.name)
	}
	return nil
}
