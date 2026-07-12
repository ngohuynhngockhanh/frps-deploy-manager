// Package store is the SQLite-backed persistence layer and the source of truth
// for servers, frp users, certs and secrets. Remote config is rendered from it.
package store

import (
	"database/sql"
	"encoding/base64"
	"fmt"

	_ "modernc.org/sqlite" // pure-Go driver, registers "sqlite"
)

var b64 = base64.StdEncoding

// Sealer encrypts/decrypts small secrets at rest (implemented by crypto.Sealer).
type Sealer interface {
	Seal([]byte) ([]byte, error)
	Open([]byte) ([]byte, error)
}

// Store wraps the database handle and exposes typed CRUD methods.
type Store struct {
	db     *sql.DB
	sealer Sealer
}

// SetSealer enables encryption-at-rest for stored secrets (dashboard passwords).
func (st *Store) SetSealer(s Sealer) { st.sealer = s }

// sealStr returns an "enc:"-prefixed sealed value; passthrough if no sealer/empty.
func (st *Store) sealStr(s string) string {
	if st.sealer == nil || s == "" {
		return s
	}
	b, err := st.sealer.Seal([]byte(s))
	if err != nil {
		return s
	}
	return "enc:" + b64.EncodeToString(b)
}

// openStr reverses sealStr; legacy plaintext (no "enc:" prefix) passes through.
func (st *Store) openStr(s string) string {
	if st.sealer == nil || len(s) < 4 || s[:4] != "enc:" {
		return s
	}
	raw, err := b64.DecodeString(s[4:])
	if err != nil {
		return s
	}
	pt, err := st.sealer.Open(raw)
	if err != nil {
		return s
	}
	return string(pt)
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
	// Purge freed pages (may retain old plaintext secrets from before sealing).
	_, _ = db.Exec("VACUUM")
	return s, nil
}

// DB exposes the raw handle for packages that need it (e.g. sessions).
func (s *Store) DB() *sql.DB { return s.db }

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }
