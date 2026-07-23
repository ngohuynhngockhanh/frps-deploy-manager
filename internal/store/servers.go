package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrNotFound is returned when a row does not exist.
var ErrNotFound = errors.New("not found")

// Server is a managed VPS target and its frps deployment parameters.
type Server struct {
	ID                int64
	Name              string
	Host              string
	SSHPort           int
	SSHUser           string
	AuthKind          string // agent | key_path | key_blob
	KeyRef            string
	HostKey           string // pinned SSH host public key (TOFU)
	Project           string
	RemoteBase        string
	Domain            string
	BindPort          int
	DashboardPort     int
	VhostHTTPPort     int
	FpmuPort          int
	NatPortStart      int
	NatPortEnd        int
	MaxPortsPerClient int
	// Optional extras (frps.ini). Zero/false = omitted.
	BindUDPPort           int
	VhostHTTPSPort        int
	TCPMux                bool
	TCPMuxHTTPConnectPort int
	DashUser              string
	DashPassword          string
	CertEmail             string
	CFToken               string // per-server Cloudflare token (sealed at rest); "" = use global
	Domain2               string // optional secondary domain on the same frps instance
	CFToken2              string // Cloudflare token for domain2's zone (sealed at rest)
	FrpsVersion           string
	Status                string
	LastDeployedAt        int64
	CreatedAt             int64
}

// RemotePath returns the deploy directory, e.g. /root/inut-jsc/frps.
func (s Server) RemotePath() string { return s.RemoteBase + "/" + s.Project }

const serverCols = `id, name, host, ssh_port, ssh_user, auth_kind, key_ref, host_key,
	project, remote_base, domain, bind_port, dashboard_port, vhost_http_port, fpmu_port,
	dash_user, dash_password, cert_email, frps_version, status, last_deployed_at, created_at,
	bind_udp_port, vhost_https_port, tcp_mux, tcpmux_httpconnect_port, cf_token,
	domain2, cf_token2, nat_port_start, nat_port_end, max_ports_per_client`

func scanServer(row interface{ Scan(...any) error }) (Server, error) {
	var s Server
	var tcpMux int
	err := row.Scan(&s.ID, &s.Name, &s.Host, &s.SSHPort, &s.SSHUser, &s.AuthKind, &s.KeyRef,
		&s.HostKey, &s.Project, &s.RemoteBase, &s.Domain, &s.BindPort, &s.DashboardPort,
		&s.VhostHTTPPort, &s.FpmuPort, &s.DashUser, &s.DashPassword, &s.CertEmail,
		&s.FrpsVersion, &s.Status, &s.LastDeployedAt, &s.CreatedAt,
		&s.BindUDPPort, &s.VhostHTTPSPort, &tcpMux, &s.TCPMuxHTTPConnectPort, &s.CFToken,
		&s.Domain2, &s.CFToken2, &s.NatPortStart, &s.NatPortEnd, &s.MaxPortsPerClient)
	s.TCPMux = tcpMux != 0
	return s, err
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// CreateServer inserts a server and returns it with ID/CreatedAt populated.
func (st *Store) CreateServer(s Server) (Server, error) {
	if s.CreatedAt == 0 {
		s.CreatedAt = time.Now().Unix()
	}
	if s.Status == "" {
		s.Status = "unprovisioned"
	}
	if s.NatPortStart == 0 {
		s.NatPortStart = 10000
	}
	if s.NatPortEnd == 0 {
		s.NatPortEnd = 60000
	}
	if s.MaxPortsPerClient == 0 {
		s.MaxPortsPerClient = 50
	}
	res, err := st.db.Exec(`INSERT INTO servers
		(name, host, ssh_port, ssh_user, auth_kind, key_ref, host_key, project, remote_base,
		 domain, bind_port, dashboard_port, vhost_http_port, fpmu_port, dash_user, dash_password,
		 cert_email, frps_version, status, last_deployed_at, created_at,
		 bind_udp_port, vhost_https_port, tcp_mux, tcpmux_httpconnect_port, cf_token,
		 domain2, cf_token2, nat_port_start, nat_port_end, max_ports_per_client)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		s.Name, s.Host, s.SSHPort, s.SSHUser, s.AuthKind, s.KeyRef, s.HostKey, s.Project, s.RemoteBase,
		s.Domain, s.BindPort, s.DashboardPort, s.VhostHTTPPort, s.FpmuPort, s.DashUser, st.sealStr(s.DashPassword),
		s.CertEmail, s.FrpsVersion, s.Status, s.LastDeployedAt, s.CreatedAt,
		s.BindUDPPort, s.VhostHTTPSPort, b2i(s.TCPMux), s.TCPMuxHTTPConnectPort, st.sealStr(s.CFToken),
		s.Domain2, st.sealStr(s.CFToken2), s.NatPortStart, s.NatPortEnd, s.MaxPortsPerClient)
	if err != nil {
		return Server{}, fmt.Errorf("insert server: %w", err)
	}
	s.ID, _ = res.LastInsertId()
	return s, nil
}

// GetServer fetches one server by id.
func (st *Store) GetServer(id int64) (Server, error) {
	row := st.db.QueryRow(`SELECT `+serverCols+` FROM servers WHERE id = ?`, id)
	s, err := scanServer(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Server{}, ErrNotFound
	}
	s.DashPassword = st.openStr(s.DashPassword)
	s.CFToken = st.openStr(s.CFToken)
	s.CFToken2 = st.openStr(s.CFToken2)
	return s, err
}

// ListServers returns all servers ordered by name.
func (st *Store) ListServers() ([]Server, error) {
	rows, err := st.db.Query(`SELECT ` + serverCols + ` FROM servers ORDER BY name, id`)
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
		s.DashPassword = st.openStr(s.DashPassword)
		s.CFToken = st.openStr(s.CFToken)
		s.CFToken2 = st.openStr(s.CFToken2)
		out = append(out, s)
	}
	return out, rows.Err()
}

// UpdateServer overwrites the mutable fields of an existing server.
func (st *Store) UpdateServer(s Server) error {
	res, err := st.db.Exec(`UPDATE servers SET
		name=?, host=?, ssh_port=?, ssh_user=?, auth_kind=?, key_ref=?, host_key=?, project=?,
		remote_base=?, domain=?, bind_port=?, dashboard_port=?, vhost_http_port=?, fpmu_port=?,
		dash_user=?, dash_password=?, cert_email=?, frps_version=?, status=?, last_deployed_at=?,
		bind_udp_port=?, vhost_https_port=?, tcp_mux=?, tcpmux_httpconnect_port=?, cf_token=?,
		domain2=?, cf_token2=?, nat_port_start=?, nat_port_end=?, max_ports_per_client=?
		WHERE id=?`,
		s.Name, s.Host, s.SSHPort, s.SSHUser, s.AuthKind, s.KeyRef, s.HostKey, s.Project,
		s.RemoteBase, s.Domain, s.BindPort, s.DashboardPort, s.VhostHTTPPort, s.FpmuPort,
		s.DashUser, st.sealStr(s.DashPassword), s.CertEmail, s.FrpsVersion, s.Status, s.LastDeployedAt,
		s.BindUDPPort, s.VhostHTTPSPort, b2i(s.TCPMux), s.TCPMuxHTTPConnectPort, st.sealStr(s.CFToken),
		s.Domain2, st.sealStr(s.CFToken2), s.NatPortStart, s.NatPortEnd, s.MaxPortsPerClient,
		s.ID)
	if err != nil {
		return fmt.Errorf("update server: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteServer removes a server (cascades to its users/certs/events).
func (st *Store) DeleteServer(id int64) error {
	res, err := st.db.Exec(`DELETE FROM servers WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
