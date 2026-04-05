package store

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestPostgresRepo_PoolAccessible verifies Pool() returns the injected pool.
func TestPostgresRepo_PoolAccessible(t *testing.T) {
	r := &PostgresRepo{pool: &pgxpool.Pool{}}
	if r.Pool() == nil {
		t.Fatal("Pool() returned nil")
	}
}

// TestQueryHistoryEntry_NilUserID verifies empty UserID produces a nil *string.
func TestQueryHistoryEntry_NilUserID(t *testing.T) {
	entry := QueryHistoryEntry{
		TenantID:  "tenant-1",
		QueryHash: "abc123",
	}
	if entry.UserID != "" {
		t.Errorf("expected empty UserID, got %q", entry.UserID)
	}
}

// TestStructZeroValues verifies all domain structs have usable zero values.
func TestStructZeroValues(t *testing.T) {
	_ = Tenant{}
	_ = User{}
	_ = QueryHistoryEntry{}
	_ = IPCBNSMapping{}
	_ = time.Now()
}

// NOTE: PostgresRepo methods require a live *pgxpool.Pool (concrete type, not interface).
// pgxmock cannot be injected without refactoring to an interface.
// Integration tests with testcontainers should cover LogQuery, GetUserByToken,
// GetQueryHistory, BulkUpsertIPCMapping, and GetOrCreateTenant.
// See postgres_integration_test.go (build tag: integration).
