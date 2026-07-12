// Package store is the SQLite-backed persistence layer and the source of truth
// for servers, frp users, certs and secrets. Remote config is rendered from it.
package store

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite" // pure-Go driver, registers "sqlite"
)

// Store wraps the database handle and exposes typed CRUD methods.
type Store struct {
	db *sql.DB
}

// Open opens (creating if needed) the SQLite database at path with WAL,
// foreign keys and a busy timeout, then applies migrations.
func Open(path string) (*Store, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	// SQLite serializes writers; a single connection sidesteps "database is
	// locked" for this single-operator tool at negligible cost.
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// DB exposes the raw handle for packages that need it (e.g. sessions).
func (s *Store) DB() *sql.DB { return s.db }

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }
