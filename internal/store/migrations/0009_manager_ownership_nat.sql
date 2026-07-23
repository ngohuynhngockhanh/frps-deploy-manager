-- Lock the application to one root admin, track FRP-user ownership, and add
-- server-wide NAT safety limits.
UPDATE app_users
SET role = 'manager'
WHERE role = 'admin'
  AND id <> (SELECT MIN(id) FROM app_users WHERE role = 'admin');

CREATE UNIQUE INDEX idx_app_users_single_admin
ON app_users(role)
WHERE role = 'admin';

ALTER TABLE frp_users
ADD COLUMN created_by_user_id INTEGER REFERENCES app_users(id) ON DELETE SET NULL;

CREATE INDEX idx_frp_users_creator ON frp_users(created_by_user_id);

ALTER TABLE servers ADD COLUMN nat_port_start INTEGER NOT NULL DEFAULT 10000;
ALTER TABLE servers ADD COLUMN nat_port_end INTEGER NOT NULL DEFAULT 60000;
ALTER TABLE servers ADD COLUMN max_ports_per_client INTEGER NOT NULL DEFAULT 50;
