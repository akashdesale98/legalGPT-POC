package store

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Tenant represents a LegalGPT tenant.
type Tenant struct {
	ID        string
	Name      string
	Slug      string
	Plan      string
	CreatedAt time.Time
}

// User represents an authenticated user.
type User struct {
	ID          string
	TenantID    string
	Email       string
	Role        string
	HashedToken string
	LastSeen    *time.Time
}

// QueryHistoryEntry is a record written after each query.
type QueryHistoryEntry struct {
	TenantID          string
	UserID            string // empty string = NULL
	QueryHash         string
	ResponseText      string
	ProviderUsed      string
	TokensIn          int
	TokensOut         int
	LatencyMs         int
	FaithfulnessScore float64
}

// IPCBNSMapping maps an IPC section to its BNS replacement.
type IPCBNSMapping struct {
	IPCSection    string
	BNSSection    string
	Description   string
	EffectiveDate string // "YYYY-MM-DD" or empty
}

// PostgresRepo is the application-layer repository backed by PostgreSQL.
// Uses pgx/v5 with a connection pool. No GORM, no database/sql.
type PostgresRepo struct {
	pool *pgxpool.Pool
}

// NewPostgresRepo opens a pgxpool connection to dsn.
// MaxConns=20, MinConns=2 per the plan spec.
func NewPostgresRepo(ctx context.Context, dsn string) (*PostgresRepo, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("postgres: parse config: %w", err)
	}
	cfg.MaxConns = 20
	cfg.MinConns = 2

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("postgres: connect: %w", err)
	}
	return &PostgresRepo{pool: pool}, nil
}

// Ping verifies the connection is alive. Used by health check.
func (r *PostgresRepo) Ping(ctx context.Context) error {
	return r.pool.Ping(ctx)
}

// Pool returns the underlying pgxpool, needed by the migration runner.
func (r *PostgresRepo) Pool() *pgxpool.Pool {
	return r.pool
}

// Close releases pool resources.
func (r *PostgresRepo) Close() {
	r.pool.Close()
}

// GetOrCreateTenant fetches a tenant by slug, inserting one if absent.
func (r *PostgresRepo) GetOrCreateTenant(ctx context.Context, slug, name, plan string) (*Tenant, error) {
	start := time.Now()

	const q = `
		INSERT INTO tenants (slug, name, plan)
		VALUES ($1, $2, $3)
		ON CONFLICT (slug) DO UPDATE SET updated_at = NOW()
		RETURNING id, name, slug, plan, created_at`

	row := r.pool.QueryRow(ctx, q, slug, name, plan)
	var t Tenant
	if err := row.Scan(&t.ID, &t.Name, &t.Slug, &t.Plan, &t.CreatedAt); err != nil {
		return nil, fmt.Errorf("postgres: GetOrCreateTenant: %w", err)
	}
	log.Printf("DB: GetOrCreateTenant duration_ms=%d slug=%s", time.Since(start).Milliseconds(), slug)
	return &t, nil
}

// GetUserByToken looks up a user whose hashed_token matches hashedToken.
// The caller is responsible for bcrypt comparison before calling this.
func (r *PostgresRepo) GetUserByToken(ctx context.Context, hashedToken string) (*User, error) {
	start := time.Now()

	const q = `
		SELECT id, tenant_id, email, role, hashed_token, last_seen
		FROM users
		WHERE hashed_token = $1
		LIMIT 1`

	row := r.pool.QueryRow(ctx, q, hashedToken)
	var u User
	if err := row.Scan(&u.ID, &u.TenantID, &u.Email, &u.Role, &u.HashedToken, &u.LastSeen); err != nil {
		return nil, fmt.Errorf("postgres: GetUserByToken: %w", err)
	}
	log.Printf("DB: GetUserByToken duration_ms=%d", time.Since(start).Milliseconds())
	return &u, nil
}

// GetUserByEmailAndTenant fetches a user by tenant_id + email.
// Used during token issuance to validate credentials.
func (r *PostgresRepo) GetUserByEmailAndTenant(ctx context.Context, tenantID, email string) (*User, error) {
	start := time.Now()

	const q = `
		SELECT id, tenant_id, email, role, hashed_token, last_seen
		FROM users
		WHERE tenant_id = $1 AND email = $2
		LIMIT 1`

	row := r.pool.QueryRow(ctx, q, tenantID, email)
	var u User
	if err := row.Scan(&u.ID, &u.TenantID, &u.Email, &u.Role, &u.HashedToken, &u.LastSeen); err != nil {
		return nil, fmt.Errorf("postgres: GetUserByEmailAndTenant: %w", err)
	}
	log.Printf("DB: GetUserByEmailAndTenant duration_ms=%d", time.Since(start).Milliseconds())
	return &u, nil
}

// LogQuery inserts a query history record.
func (r *PostgresRepo) LogQuery(ctx context.Context, entry QueryHistoryEntry) error {
	start := time.Now()

	var userID *string
	if entry.UserID != "" {
		userID = &entry.UserID
	}

	const q = `
		INSERT INTO query_history
			(tenant_id, user_id, query_hash, response_text, provider_used,
			 tokens_in, tokens_out, latency_ms, faithfulness_score)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	_, err := r.pool.Exec(ctx, q,
		entry.TenantID, userID, entry.QueryHash, entry.ResponseText,
		entry.ProviderUsed, entry.TokensIn, entry.TokensOut,
		entry.LatencyMs, entry.FaithfulnessScore,
	)
	if err != nil {
		return fmt.Errorf("postgres: LogQuery: %w", err)
	}
	log.Printf("DB: LogQuery duration_ms=%d", time.Since(start).Milliseconds())
	return nil
}

// GetQueryHistory returns up to limit records for the given tenant, newest first.
func (r *PostgresRepo) GetQueryHistory(ctx context.Context, tenantID string, limit int) ([]QueryHistoryEntry, error) {
	start := time.Now()

	const q = `
		SELECT tenant_id, COALESCE(user_id::text, ''), query_hash,
		       COALESCE(response_text, ''), COALESCE(provider_used, ''),
		       COALESCE(tokens_in, 0), COALESCE(tokens_out, 0),
		       COALESCE(latency_ms, 0), COALESCE(faithfulness_score, 0)
		FROM query_history
		WHERE tenant_id = $1
		ORDER BY created_at DESC
		LIMIT $2`

	rows, err := r.pool.Query(ctx, q, tenantID, limit)
	if err != nil {
		return nil, fmt.Errorf("postgres: GetQueryHistory: %w", err)
	}
	defer rows.Close()

	entries := make([]QueryHistoryEntry, 0)
	for rows.Next() {
		var e QueryHistoryEntry
		if err := rows.Scan(
			&e.TenantID, &e.UserID, &e.QueryHash, &e.ResponseText,
			&e.ProviderUsed, &e.TokensIn, &e.TokensOut,
			&e.LatencyMs, &e.FaithfulnessScore,
		); err != nil {
			return nil, fmt.Errorf("postgres: GetQueryHistory scan: %w", err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: GetQueryHistory iteration: %w", err)
	}
	log.Printf("DB: GetQueryHistory duration_ms=%d count=%d", time.Since(start).Milliseconds(), len(entries))
	return entries, nil
}

// BulkUpsertIPCMapping inserts or updates IPC→BNS mappings within a single transaction.
// ON CONFLICT updates bns_section to keep data fresh on re-ingest.
func (r *PostgresRepo) BulkUpsertIPCMapping(ctx context.Context, mappings []IPCBNSMapping) error {
	start := time.Now()

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("postgres: BulkUpsertIPCMapping begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after commit is harmless

	const q = `
		INSERT INTO ipc_bns_map (ipc_section, bns_section, description, effective_date)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (ipc_section)
		DO UPDATE SET
			bns_section    = EXCLUDED.bns_section,
			description    = EXCLUDED.description,
			effective_date = EXCLUDED.effective_date`

	for _, m := range mappings {
		var effDate *string
		if m.EffectiveDate != "" {
			effDate = &m.EffectiveDate
		}
		if _, err := tx.Exec(ctx, q, m.IPCSection, m.BNSSection, m.Description, effDate); err != nil {
			return fmt.Errorf("postgres: BulkUpsertIPCMapping ipc=%s: %w", m.IPCSection, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("postgres: BulkUpsertIPCMapping commit: %w", err)
	}
	log.Printf("DB: ipc_bns_map loaded count=%d duration_ms=%d", len(mappings), time.Since(start).Milliseconds())
	return nil
}
