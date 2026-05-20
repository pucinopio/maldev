// Package store wraps the ENT-generated client with a constructor that opens
// a SQLite file via the pure-Go modernc.org/sqlite driver.
package store

import (
	"context"
	stdsql "database/sql"
	"fmt"

	"entgo.io/ent/dialect"
	entdialectsql "entgo.io/ent/dialect/sql"

	_ "modernc.org/sqlite"

	"github.com/oioio-space/maldev/internal/manager/store/ent"
)

// Store wraps the ENT client.
type Store struct {
	*ent.Client
}

// New opens (or creates) a SQLite store at path and runs schema migrations.
// Pass ":memory:" or "file::memory:?cache=shared" for an in-memory store
// (used by tests).
func New(ctx context.Context, path string) (*Store, error) {
	// modernc.org/sqlite registers under the "sqlite" driver name, while ENT's
	// dialect.SQLite constant is "sqlite3". Open via database/sql directly so
	// we control the driver name, then hand the *sql.DB to ENT as sqlite3 dialect.
	dsn := "file:" + path + "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
	db, err := stdsql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("store: open %s: %w", path, err)
	}
	drv := entdialectsql.OpenDB(dialect.SQLite, db)
	client := ent.NewClient(ent.Driver(drv))
	if err := client.Schema.Create(ctx); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("store: migrate: %w", err)
	}
	return &Store{Client: client}, nil
}

// Close closes the underlying ENT client (and SQLite connection).
func (s *Store) Close() error { return s.Client.Close() }
