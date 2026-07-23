# frps-deploy-manager

Local admin tool to **deploy and manage [frps](https://github.com/fatedier/frp) (frp server)** instances on remote VPS hosts over SSH — from the operator's workstation, through a web UI.

- Single Go binary, embedded UI (Go `html/template` + Tailwind + htmx).
- Listens **loopback-only** on `127.0.0.1:2050`, behind an admin login.
- Deploys frps + the [`fp-multiuser`](https://github.com/gofrp/fp-multiuser) auth plugin, manages frps users, nginx reverse-proxy, and Let's Encrypt certificates (with expiry tracking).
- SQLite (`modernc.org/sqlite`, pure-Go) as the source of truth; remote config is rendered from the DB → idempotent re-deploys.

## Quick start

```bash
make bootstrap-go     # one-time: fetch Go toolchain into ~/go-sdk
make build            # -> bin/frps-deploy-manager
./bin/frps-deploy-manager --version
make run              # serve on http://127.0.0.1:2050  (admin / NhapHang123@)
```

## Layout

| Path | Purpose |
|------|---------|
| `cmd/frps-deploy-manager` | main entrypoint |
| `internal/store` | SQLite persistence + migrations |
| `internal/auth` | local UI login (argon2id, sessions, CSRF) |
| `internal/ssh` | SSH/SFTP primitives (TOFU host keys) |
| `internal/remote` | high-level remote ops + config templates |
| `internal/builder` | cross-compile frps & fp-multiuser |
| `internal/httpapi` | REST + page handlers (SSE deploy logs) |
| `internal/web` | embedded templates + static assets |
| `third_party/fp-multiuser` | vendored auth plugin (nested Go module) |

See `docs/runbook.md` for deployment/operations and `docs/architecture.md` for
the RBAC model, NAT safety boundary, data flow, and maintenance rules.
