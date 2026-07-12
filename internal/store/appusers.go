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
	ID                 int64
	Username           string
	PwHash             string
	Role               string
	MustChangePassword bool
	CreatedAt          int64
}

const appUserCols = `id, username, pw_hash, role, must_change_password, created_at`

func scanAppUser(row interface{ Scan(...any) error }) (AppUser, error) {
	var u AppUser
	var mc int
	err := row.Scan(&u.ID, &u.Username, &u.PwHash, &u.Role, &mc, &u.CreatedAt)
	u.MustChangePassword = mc != 0
	return u, err
}

// CountAppUsers returns how many operator accounts exist (for admin seeding).
func (st *Store) CountAppUsers() (int, error) {
	var n int
	err := st.db.QueryRow(`SELECT COUNT(*) FROM app_users`).Scan(&n)
	return n, err
}

// CreateAppUser inserts an operator account. mustChange forces a password change
// on first login (temp/known password).
func (st *Store) CreateAppUser(username, pwHash, role string, mustChange bool) (AppUser, error) {
	if role == "" {
		role = RoleManager
	}
	u := AppUser{Username: username, PwHash: pwHash, Role: role, MustChangePassword: mustChange, CreatedAt: time.Now().Unix()}
	mc := 0
	if mustChange {
		mc = 1
	}
	res, err := st.db.Exec(`INSERT INTO app_users(username, pw_hash, role, must_change_password, created_at) VALUES(?,?,?,?,?)`,
		u.Username, u.PwHash, u.Role, mc, u.CreatedAt)
	if err != nil {
		return AppUser{}, err
	}
	u.ID, _ = res.LastInsertId()
	return u, nil
}

// GetAppUserByUsername looks up an operator account by name.
func (st *Store) GetAppUserByUsername(username string) (AppUser, error) {
	u, err := scanAppUser(st.db.QueryRow(`SELECT `+appUserCols+` FROM app_users WHERE username = ?`, username))
	if errors.Is(err, sql.ErrNoRows) {
		return AppUser{}, ErrNotFound
	}
	return u, err
}

// GetAppUser looks up an operator account by id.
func (st *Store) GetAppUser(id int64) (AppUser, error) {
	u, err := scanAppUser(st.db.QueryRow(`SELECT `+appUserCols+` FROM app_users WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return AppUser{}, ErrNotFound
	}
	return u, err
}

// ListAppUsers returns all operator accounts ordered by username.
func (st *Store) ListAppUsers() ([]AppUser, error) {
	rows, err := st.db.Query(`SELECT ` + appUserCols + ` FROM app_users ORDER BY role, username`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AppUser
	for rows.Next() {
		u, err := scanAppUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// SetAppUserPassword updates an operator's password hash and clears the
// must-change flag (they just changed it).
func (st *Store) SetAppUserPassword(id int64, pwHash string) error {
	_, err := st.db.Exec(`UPDATE app_users SET pw_hash = ?, must_change_password = 0 WHERE id = ?`, pwHash, id)
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
