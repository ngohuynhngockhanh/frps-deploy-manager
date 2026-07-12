package store

// SetUserServers replaces a manager's assigned servers (sites) with the given
// set, in a transaction.
func (st *Store) SetUserServers(userID int64, serverIDs []int64) error {
	tx, err := st.db.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM user_servers WHERE user_id = ?`, userID); err != nil {
		_ = tx.Rollback()
		return err
	}
	for _, sid := range serverIDs {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO user_servers(user_id, server_id) VALUES(?,?)`, userID, sid); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// UserServerIDs returns the server ids a user is assigned to.
func (st *Store) UserServerIDs(userID int64) (map[int64]bool, error) {
	rows, err := st.db.Query(`SELECT server_id FROM user_servers WHERE user_id = ?`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]bool{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

// UserCanAccess reports whether a user is assigned to a server.
func (st *Store) UserCanAccess(userID, serverID int64) (bool, error) {
	var n int
	err := st.db.QueryRow(`SELECT COUNT(*) FROM user_servers WHERE user_id = ? AND server_id = ?`, userID, serverID).Scan(&n)
	return n > 0, err
}

// ListServersForUser returns the servers a manager may see (their assigned set).
func (st *Store) ListServersForUser(userID int64) ([]Server, error) {
	rows, err := st.db.Query(`SELECT `+serverCols+` FROM servers
		WHERE id IN (SELECT server_id FROM user_servers WHERE user_id = ?) ORDER BY name, id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Server
	for rows.Next() {
		s, err := scanServer(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
