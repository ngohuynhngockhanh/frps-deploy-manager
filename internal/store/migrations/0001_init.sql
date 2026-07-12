-- Initial schema. Timestamps are unix seconds (INTEGER).

CREATE TABLE app_users (
  id         INTEGER PRIMARY KEY,
  username   TEXT UNIQUE NOT NULL,
  pw_hash    TEXT NOT NULL,            -- argon2id PHC string
  created_at INTEGER NOT NULL
);

CREATE TABLE servers (
  id               INTEGER PRIMARY KEY,
  name             TEXT NOT NULL,
  host             TEXT NOT NULL,
  ssh_port         INTEGER NOT NULL DEFAULT 22,
  ssh_user         TEXT NOT NULL DEFAULT 'root',
  auth_kind        TEXT NOT NULL DEFAULT 'agent',  -- 'agent' | 'key_path' | 'key_blob'
  key_ref          TEXT NOT NULL DEFAULT '',       -- path, or secret id
  host_key         TEXT NOT NULL DEFAULT '',       -- pinned SSH host public key (TOFU)
  project          TEXT NOT NULL,                  -- 'frps'
  remote_base      TEXT NOT NULL,                  -- '/root/inut-jsc'
  domain           TEXT NOT NULL DEFAULT '',       -- io.vnmap3d.com
  bind_port        INTEGER NOT NULL DEFAULT 9000,
  dashboard_port   INTEGER NOT NULL DEFAULT 9001,
  vhost_http_port  INTEGER NOT NULL DEFAULT 9002,
  fpmu_port        INTEGER NOT NULL DEFAULT 9003,
  dash_user        TEXT NOT NULL DEFAULT 'admin',
  dash_password    TEXT NOT NULL DEFAULT '',
  cert_email       TEXT NOT NULL DEFAULT '',
  frps_version     TEXT NOT NULL DEFAULT '',
  status           TEXT NOT NULL DEFAULT 'unprovisioned',
  last_deployed_at INTEGER NOT NULL DEFAULT 0,
  created_at       INTEGER NOT NULL,
  UNIQUE(host, project)
);

CREATE TABLE frp_users (
  id         INTEGER PRIMARY KEY,
  server_id  INTEGER NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
  username   TEXT NOT NULL,           -- frpc 'user'
  token_enc  BLOB NOT NULL,           -- AES-GCM encrypted token
  enabled    INTEGER NOT NULL DEFAULT 1,
  created_at INTEGER NOT NULL,
  UNIQUE(server_id, username)
);

CREATE TABLE certs (
  id           INTEGER PRIMARY KEY,
  server_id    INTEGER NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
  domain       TEXT NOT NULL,
  issuer       TEXT NOT NULL DEFAULT '',
  not_before   INTEGER NOT NULL DEFAULT 0,
  not_after    INTEGER NOT NULL DEFAULT 0,
  serial       TEXT NOT NULL DEFAULT '',
  cert_path    TEXT NOT NULL DEFAULT '',
  last_checked INTEGER NOT NULL DEFAULT 0,
  UNIQUE(server_id, domain)
);

CREATE TABLE secrets (
  id         INTEGER PRIMARY KEY,
  kind       TEXT NOT NULL,
  nonce      BLOB NOT NULL,
  ciphertext BLOB NOT NULL,
  created_at INTEGER NOT NULL
);

CREATE TABLE deploy_events (
  id         INTEGER PRIMARY KEY,
  server_id  INTEGER REFERENCES servers(id) ON DELETE CASCADE,
  phase      TEXT NOT NULL,
  ok         INTEGER NOT NULL,
  detail     TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL
);

CREATE INDEX idx_frp_users_server ON frp_users(server_id);
CREATE INDEX idx_certs_server ON certs(server_id);
CREATE INDEX idx_deploy_events_server ON deploy_events(server_id, id DESC);
