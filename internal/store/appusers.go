package store

import (
	"database/sql"
	"errors"
	"time"
)

// Roles for operator accounts.
const (
	RoleAdmin   = "admin"   // full access
	RoleManager = "manager" // scoped to assigned servers; manage frp users only
)

// AppUser is a local UI operator account.
type AppUser struct {
	ID        int64
	Username  string
	PwHash    string
	Role      string
	CreatedAt int64
}

// CountAppUsers returns how many operator accounts exist (for admin seeding).
func (st *Store) CountAppUsers() (int, error) {
	var n int
	err := st.db.QueryRow(`SELECT COUNT(*) FROM app_users`).Scan(&n)
	return n, err
}

// CreateAppUser inserts an operator account with a role.
func (st *Store) CreateAppUser(username, pwHash, role string) (AppUser, error) {
	if role == "" {
		role = RoleManager
	}
	u := AppUser{Username: username, PwHash: pwHash, Role: role, CreatedAt: time.Now().Unix()}
	res, err := st.db.Exec(`INSERT INTO app_users(username, pw_hash, role, created_at) VALUES(?,?,?,?)`,
		u.Username, u.PwHash, u.Role, u.CreatedAt)
	if err != nil {
		return AppUser{}, err
	}
	u.ID, _ = res.LastInsertId()
	return u, nil
}

// GetAppUserByUsername looks up an operator account by name.
func (st *Store) GetAppUserByUsername(username string) (AppUser, error) {
	var u AppUser
	err := st.db.QueryRow(`SELECT id, username, pw_hash, role, created_at FROM app_users WHERE username = ?`, username).
		Scan(&u.ID, &u.Username, &u.PwHash, &u.Role, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return AppUser{}, ErrNotFound
	}
	return u, err
}

// GetAppUser looks up an operator account by id.
func (st *Store) GetAppUser(id int64) (AppUser, error) {
	var u AppUser
	err := st.db.QueryRow(`SELECT id, username, pw_hash, role, created_at FROM app_users WHERE id = ?`, id).
		Scan(&u.ID, &u.Username, &u.PwHash, &u.Role, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return AppUser{}, ErrNotFound
	}
	return u, err
}

// ListAppUsers returns all operator accounts ordered by username.
func (st *Store) ListAppUsers() ([]AppUser, error) {
	rows, err := st.db.Query(`SELECT id, username, pw_hash, role, created_at FROM app_users ORDER BY role, username`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AppUser
	for rows.Next() {
		var u AppUser
		if err := rows.Scan(&u.ID, &u.Username, &u.PwHash, &u.Role, &u.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// SetAppUserPassword updates an operator's password hash.
func (st *Store) SetAppUserPassword(id int64, pwHash string) error {
	_, err := st.db.Exec(`UPDATE app_users SET pw_hash = ? WHERE id = ?`, pwHash, id)
	return err
}

// DeleteAppUser removes an operator account.
func (st *Store) DeleteAppUser(id int64) error {
	res, err := st.db.Exec(`DELETE FROM app_users WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
