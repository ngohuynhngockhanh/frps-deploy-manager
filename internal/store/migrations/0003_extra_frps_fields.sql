-- Optional extra frps.ini settings (used e.g. by the p2p standard).
ALTER TABLE servers ADD COLUMN bind_udp_port INTEGER NOT NULL DEFAULT 0;
ALTER TABLE servers ADD COLUMN vhost_https_port INTEGER NOT NULL DEFAULT 0;
ALTER TABLE servers ADD COLUMN tcp_mux INTEGER NOT NULL DEFAULT 0;
ALTER TABLE servers ADD COLUMN tcpmux_httpconnect_port INTEGER NOT NULL DEFAULT 0;
