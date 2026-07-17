-- Optional secondary domain served by the SAME frps instance (e.g. a second
-- Cloudflare zone pointed at the same VPS). Each has its own sealed CF token so
-- the cert buttons operate on the right zone. Empty = no secondary domain.
ALTER TABLE servers ADD COLUMN domain2 TEXT NOT NULL DEFAULT '';
ALTER TABLE servers ADD COLUMN cf_token2 TEXT NOT NULL DEFAULT '';
