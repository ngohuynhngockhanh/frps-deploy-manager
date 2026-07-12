package store

import (
	"database/sql"
	"errors"
	"time"
)

// AppUser is a local UI operator account.
type AppUser struct {
	ID        int64
	Username  string
	PwHash    string
	CreatedAt int64
}

// CountAppUsers returns how many operator accounts exist (for admin seeding).
func (st *Store) CountAppUsers() (int, error) {
	var n int
	err := st.db.QueryRow(`SELECT COUNT(*) FROM app_users`).Scan(&n)
	return n, err
}

// CreateAppUser inserts an operator account.
func (st *Store) CreateAppUser(username, pwHash string) (AppUser, error) {
	u := AppUser{Username: username, PwHash: pwHash, CreatedAt: time.Now().Unix()}
	res, err := st.db.Exec(`INSERT INTO app_users(username, pw_hash, created_at) VALUES(?,?,?)`,
		u.Username, u.PwHash, u.CreatedAt)
	if err != nil {
		return AppUser{}, err
	}
	u.ID, _ = res.LastInsertId()
	return u, nil
}

// GetAppUserByUsername looks up an operator account by name.
func (st *Store) GetAppUserByUsername(username string) (AppUser, error) {
	var u AppUser
	err := st.db.QueryRow(`SELECT id, username, pw_hash, created_at FROM app_users WHERE username = ?`, username).
		Scan(&u.ID, &u.Username, &u.PwHash, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return AppUser{}, ErrNotFound
	}
	return u, err
}

// SetAppUserPassword updates an operator's password hash.
func (st *Store) SetAppUserPassword(id int64, pwHash string) error {
	_, err := st.db.Exec(`UPDATE app_users SET pw_hash = ? WHERE id = ?`, pwHash, id)
	return err
}
