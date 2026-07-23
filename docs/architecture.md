# frps-deploy-manager architecture

This document is the map for maintaining the application after deployment.

## System shape

```text
Browser
  -> loopback HTTP (Go templates + htmx)
  -> auth middleware (session, CSRF, role, assigned server)
  -> HTTP handlers
       -> SQLite store (source of truth)
       -> SSH deployer -> remote VPS
       -> FRPS dashboard API (sanitized status only for managers)

Remote VPS
  nginx :443
    -> FRPS dashboard on 127.0.0.1:<dashboard_port>
    -> FRPS vhost HTTP on 127.0.0.1:<vhost_http_port>
  FRPS <bind_port>
    -> fp-multiuser Login plugin on 127.0.0.1:<fpmu_port>
```

The manager binary embeds templates, CSS, JavaScript, migrations, and remote
configuration templates. SQLite under `data/` remains the source of truth;
remote configuration is regenerated from the database on deploy or FP-user
sync.

## Trust and roles

There is exactly one `admin` account. Migration `0009` keeps the oldest admin,
demotes any additional admins to `manager`, and creates a partial unique index
that prevents a second admin from being inserted.

| Capability | Admin | Manager |
|---|---:|---:|
| View all servers | Yes | Assigned servers only |
| Configure/deploy/delete server | Yes | No |
| Manage certificates and Cloudflare | Yes | Read status only |
| Open full FRPS dashboard | Yes | No |
| View sanitized live FRPS statistics | Yes | Yes, assigned servers only |
| Create FP users | Yes | Yes, assigned servers only |
| View/rotate/delete any FP user | Yes | No |
| View/rotate/delete self-created FP user | Yes | Yes |
| Create or assign manager accounts | Yes | No |

`frp_users.created_by_user_id` records ownership. Legacy rows and rows whose
manager was deleted have a null owner and are admin-managed. Deleting a manager
never deletes their FP users. Manager authorization is checked in handlers,
not only by hiding buttons in templates.

Password reset and account deletion invalidate all cached and persisted
sessions for that username. New accounts are always created as managers even
if a crafted request submits `role=admin`.

## NAT safety

Each server stores:

- `nat_port_start` (default `10000`)
- `nat_port_end` (default `60000`)
- `max_ports_per_client` (default `50`)

Deploy renders these as legacy FRPS settings:

```ini
allow_ports = 10000-60000
max_ports_per_client = 50
```

FRPS rejects TCP/UDP proxies outside the allowed range and rejects a port that
is already active. This is a shared range, not permanent port ownership: after
a tunnel releases a port, another authenticated FP user may use it. The server
form rejects invalid ranges and ranges overlapping FRPS control, dashboard,
vhost, plugin, or optional extra ports.

## Live status boundary

`GET /servers/{id}/status` calls `https://<domain>/api/serverinfo` with the
stored FRPS dashboard credentials, a six-second timeout, and a one-megabyte
response limit. The rendered partial exposes only aggregate operational data:
version, client count, current connections, traffic, proxy counts, and allowed
ports. Managers never receive dashboard credentials or raw proxy details.

The full `/dash/` and proxied `/api/` namespaces are admin-only.

## Data and secrets

Back up the entire `data/` directory together:

- `app.db`: servers, accounts, assignments, FP users, sessions, cert metadata
- `master.key`: required to decrypt dashboard passwords, Cloudflare tokens,
  and FP-user tokens
- `known_hosts`: pinned SSH host keys
- `build-cache/`: disposable cross-compiled artifacts

Losing `master.key` makes encrypted secrets unrecoverable. Copying only the
database is not a usable secret backup.

## Change workflow

1. Add forward-only SQL migrations under `internal/store/migrations/`.
2. Update typed store models and tests in the same change.
3. Keep authorization checks server-side before remote sync or mutation.
4. Edit `internal/web/tailwind/input.css`, then run `make ui` to regenerate the
   embedded `internal/web/static/tailwind.css`.
5. Run `make test`, `make vet`, and `make build` before deployment.

