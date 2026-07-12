-- Record which operator performed each deploy/cert action (audit trail).
ALTER TABLE deploy_events ADD COLUMN actor TEXT NOT NULL DEFAULT '';
