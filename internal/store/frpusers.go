package store

import (
	"database/sql"
	"errors"
	"time"
)

// FrpUser is an frps auth user (fp-multiuser). The token is stored encrypted
// (token_enc = nonce||ciphertext) and never exposed in this struct.
type FrpUser struct {
	ID          int64
	ServerID    int64
	Username    string
	Enabled     bool
	CreatedByID int64
	CreatorName string
	CreatedAt   int64
}

// AddFrpUser inserts an frp user with an already-sealed token blob.
func (st *Store) AddFrpUser(serverID int64, username string, tokenEnc []byte, createdByID int64) (FrpUser, error) {
	u := FrpUser{ServerID: serverID, Username: username, Enabled: true, CreatedByID: createdByID, CreatedAt: time.Now().Unix()}
	var creator any
	if createdByID != 0 {
		creator = createdByID
	}
	res, err := st.db.Exec(`INSERT INTO frp_users(server_id, username, token_enc, enabled, created_at, created_by_user_id)
		VALUES(?,?,?,?,?,?)`, serverID, username, tokenEnc, 1, u.CreatedAt, creator)
	if err != nil {
		return FrpUser{}, err
	}
	u.ID, _ = res.LastInsertId()
	return u, nil
}

// ListFrpUsers returns the (metadata-only) users for a server, ordered by name.
func (st *Store) ListFrpUsers(serverID int64) ([]FrpUser, error) {
	rows, err := st.db.Query(`SELECT f.id, f.server_id, f.username, f.enabled,
		f.created_by_user_id, COALESCE(a.username, ''), f.created_at
		FROM frp_users f LEFT JOIN app_users a ON a.id = f.created_by_user_id
		WHERE f.server_id = ? ORDER BY f.username`, serverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FrpUser
	for rows.Next() {
		var u FrpUser
		var en int
		var creator sql.NullInt64
		if err := rows.Scan(&u.ID, &u.ServerID, &u.Username, &en, &creator, &u.CreatorName, &u.CreatedAt); err != nil {
			return nil, err
		}
		u.Enabled = en != 0
		if creator.Valid {
			u.CreatedByID = creator.Int64
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// GetFrpUser returns one user record (metadata only).
func (st *Store) GetFrpUser(id int64) (FrpUser, error) {
	var u FrpUser
	var en int
	var creator sql.NullInt64
	err := st.db.QueryRow(`SELECT f.id, f.server_id, f.username, f.enabled,
		f.created_by_user_id, COALESCE(a.username, ''), f.created_at
		FROM frp_users f LEFT JOIN app_users a ON a.id = f.created_by_user_id WHERE f.id = ?`, id).
		Scan(&u.ID, &u.ServerID, &u.Username, &en, &creator, &u.CreatorName, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return FrpUser{}, ErrNotFound
	}
	u.Enabled = en != 0
	if creator.Valid {
		u.CreatedByID = creator.Int64
	}
	return u, err
}

// FrpTokenBlobs returns username -> sealed token blob for enabled users, used to
// render the remote tokens file.
func (st *Store) FrpTokenBlobs(serverID int64) (map[string][]byte, error) {
	rows, err := st.db.Query(`SELECT username, token_enc FROM frp_users
		WHERE server_id = ? AND enabled = 1 ORDER BY username`, serverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string][]byte)
	for rows.Next() {
		var name string
		var blob []byte
		if err := rows.Scan(&name, &blob); err != nil {
			return nil, err
		}
		out[name] = blob
	}
	return out, rows.Err()
}

// UpdateFrpToken replaces a user's sealed token (rotate).
func (st *Store) UpdateFrpToken(id int64, tokenEnc []byte) error {
	res, err := st.db.Exec(`UPDATE frp_users SET token_enc = ? WHERE id = ?`, tokenEnc, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteFrpUser removes an frp user.
func (st *Store) DeleteFrpUser(id int64) error {
	res, err := st.db.Exec(`DELETE FROM frp_users WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
