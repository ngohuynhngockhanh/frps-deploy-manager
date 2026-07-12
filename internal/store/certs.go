package store

import (
	"database/sql"
	"errors"
	"time"
)

// Cert is a tracked TLS certificate for a server's domain.
type Cert struct {
	ID          int64
	ServerID    int64
	Domain      string
	Issuer      string
	NotBefore   int64
	NotAfter    int64
	Serial      string
	CertPath    string
	LastChecked int64
}

// DaysRemaining returns whole days until expiry (negative if expired) relative
// to now.
func (c Cert) DaysRemaining() int {
	if c.NotAfter == 0 {
		return 0
	}
	return int((c.NotAfter - time.Now().Unix()) / 86400)
}

// UpsertCert inserts or updates the cert row for (server_id, domain).
func (st *Store) UpsertCert(c Cert) error {
	if c.LastChecked == 0 {
		c.LastChecked = time.Now().Unix()
	}
	_, err := st.db.Exec(`INSERT INTO certs
		(server_id, domain, issuer, not_before, not_after, serial, cert_path, last_checked)
		VALUES (?,?,?,?,?,?,?,?)
		ON CONFLICT(server_id, domain) DO UPDATE SET
			issuer=excluded.issuer, not_before=excluded.not_before, not_after=excluded.not_after,
			serial=excluded.serial, cert_path=excluded.cert_path, last_checked=excluded.last_checked`,
		c.ServerID, c.Domain, c.Issuer, c.NotBefore, c.NotAfter, c.Serial, c.CertPath, c.LastChecked)
	return err
}

// GetCert fetches a cert by server + domain.
func (st *Store) GetCert(serverID int64, domain string) (Cert, error) {
	c, err := scanCert(st.db.QueryRow(`SELECT id, server_id, domain, issuer, not_before, not_after,
		serial, cert_path, last_checked FROM certs WHERE server_id=? AND domain=?`, serverID, domain))
	if errors.Is(err, sql.ErrNoRows) {
		return Cert{}, ErrNotFound
	}
	return c, err
}

// ListCertsWithDomain returns all cert rows that have a non-empty domain across
// all servers (used by the expiry-refresh scheduler).
func (st *Store) ListCertsWithDomain() ([]Cert, error) {
	rows, err := st.db.Query(`SELECT id, server_id, domain, issuer, not_before, not_after,
		serial, cert_path, last_checked FROM certs WHERE domain != ''`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Cert
	for rows.Next() {
		c, err := scanCert(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func scanCert(row interface{ Scan(...any) error }) (Cert, error) {
	var c Cert
	err := row.Scan(&c.ID, &c.ServerID, &c.Domain, &c.Issuer, &c.NotBefore, &c.NotAfter,
		&c.Serial, &c.CertPath, &c.LastChecked)
	return c, err
}
