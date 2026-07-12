package store

import "time"

// DeployEvent is one recorded step of a provisioning run.
type DeployEvent struct {
	ID        int64
	ServerID  int64
	Phase     string
	OK        bool
	Detail    string
	CreatedAt int64
}

// AddDeployEvent records a provisioning step for audit/history.
func (st *Store) AddDeployEvent(serverID int64, phase string, ok bool, detail string) error {
	okv := 0
	if ok {
		okv = 1
	}
	_, err := st.db.Exec(`INSERT INTO deploy_events(server_id, phase, ok, detail, created_at)
		VALUES(?,?,?,?,?)`, serverID, phase, okv, detail, time.Now().Unix())
	return err
}

// RecentDeployEvents returns the latest events for a server (newest first).
func (st *Store) RecentDeployEvents(serverID int64, limit int) ([]DeployEvent, error) {
	rows, err := st.db.Query(`SELECT id, server_id, phase, ok, detail, created_at
		FROM deploy_events WHERE server_id = ? ORDER BY id DESC LIMIT ?`, serverID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DeployEvent
	for rows.Next() {
		var e DeployEvent
		var ok int
		if err := rows.Scan(&e.ID, &e.ServerID, &e.Phase, &ok, &e.Detail, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.OK = ok != 0
		out = append(out, e)
	}
	return out, rows.Err()
}
