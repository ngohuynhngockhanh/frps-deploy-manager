package store

import (
	"database/sql"
	"errors"
)

// Session is a persisted login session.
type Session struct {
	Token     string
	Username  string
	CSRF      string
	ExpiresAt int64
}

// SaveSession inserts or replaces a session.
func (st *Store) SaveSession(s Session) error {
	_, err := st.db.Exec(`INSERT INTO sessions(token, username, csrf, expires_at)
		VALUES(?,?,?,?)
		ON CONFLICT(token) DO UPDATE SET expires_at=excluded.expires_at`,
		s.Token, s.Username, s.CSRF, s.ExpiresAt)
	return err
}

// GetSession loads a session by token.
func (st *Store) GetSession(token string) (Session, error) {
	var s Session
	err := st.db.QueryRow(`SELECT token, username, csrf, expires_at FROM sessions WHERE token = ?`, token).
		Scan(&s.Token, &s.Username, &s.CSRF, &s.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrNotFound
	}
	return s, err
}

// DeleteSession removes a session (logout).
func (st *Store) DeleteSession(token string) error {
	_, err := st.db.Exec(`DELETE FROM sessions WHERE token = ?`, token)
	return err
}

// DeleteExpiredSessions purges sessions past their expiry.
func (st *Store) DeleteExpiredSessions(now int64) error {
	_, err := st.db.Exec(`DELETE FROM sessions WHERE expires_at < ?`, now)
	return err
}
