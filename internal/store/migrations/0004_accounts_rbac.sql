-- Sub-accounts with roles + per-server (site) access for managers.
ALTER TABLE app_users ADD COLUMN role TEXT NOT NULL DEFAULT 'admin';

CREATE TABLE user_servers (
  user_id   INTEGER NOT NULL REFERENCES app_users(id) ON DELETE CASCADE,
  server_id INTEGER NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
  PRIMARY KEY (user_id, server_id)
);
