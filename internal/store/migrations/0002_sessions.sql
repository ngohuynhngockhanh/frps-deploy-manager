-- Persisted login sessions so users stay logged in across restarts.
CREATE TABLE sessions (
  token      TEXT PRIMARY KEY,
  username   TEXT NOT NULL,
  csrf       TEXT NOT NULL,
  expires_at INTEGER NOT NULL
);
CREATE INDEX idx_sessions_expires ON sessions(expires_at);
