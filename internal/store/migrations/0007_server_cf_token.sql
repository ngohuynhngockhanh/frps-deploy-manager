-- Per-server Cloudflare API token (sealed at rest) so each site can use its own
-- zone's token for DNS-01 wildcard certs. Empty = fall back to the global token.
ALTER TABLE servers ADD COLUMN cf_token TEXT NOT NULL DEFAULT '';
