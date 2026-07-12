# frps-deploy-manager — Runbook

## Run locally

```bash
make bootstrap-go     # one-time: Go toolchain into ~/go-sdk
make build            # -> bin/frps-deploy-manager
make run              # http://127.0.0.1:2050  (admin / NhapHang123@)
make ui               # rebuild embedded Tailwind CSS after editing input.css
make test             # unit tests (build tests need ~/ksp-frp + toolchain)
```

Config via env (all optional): `FDM_ADDR` (default `127.0.0.1:2050`), `FDM_DATA_DIR`
(`data/`), `FDM_FRP_SRC` (`~/ksp-frp`), `FDM_FPMU_SRC` (`third_party/fp-multiuser`),
`FDM_GO_BIN` (`~/go-sdk/bin/go`), `ADMIN_INITIAL_PASSWORD`.

State lives under `data/`: `app.db` (SQLite, source of truth), `master.key`
(AES key for sealed tokens, mode 0600), `build-cache/` (cross-compiled binaries),
`known_hosts`. Back up `data/` to preserve servers, users and the master key.

## Deploy a server (UI flow)

1. **New server** → host, SSH port, domain, cert email. SSH auth defaults to
   ssh-agent, falling back to `~/.ssh/id_ed25519|id_ecdsa|id_rsa`; or choose
   `key_path` and give a key file. (SSH keys onto the VPS are your responsibility.)
2. **Test SSH & scan ports** — pins the host key (TOFU), reports nginx/certbot/
   systemd presence and any conflicts on ports 9000–9003.
3. **Deploy frps** — builds frps + fp-multiuser (cached), uploads, writes
   `frps.toml` + systemd units under `/root/inut-jsc/<project>`, starts
   fp-multiuser then frps, verifies both active and bindPort listening. A random
   dashboard password is generated and shown in the deploy log.
4. **frps users** — add/rotate/delete. Token is auto-generated if blank and
   shown once (unrecoverable after — it is stored AES-GCM sealed). Adding syncs
   `tokens.txt` to the remote and restarts fp-multiuser (live tunnels unaffected).
   frpc connects with `serverPort=<bindPort>, user=<name>, metadatas.token=<token>`.
5. **HTTPS certificate** — Issue/Renew runs `certbot --nginx --redirect
   --keep-until-expiring`; `io.vnmap3d.com` (:443) proxies to the frps dashboard
   on `127.0.0.1:<dashboardPort>`. Expiry is parsed and shown as a green/amber/red
   badge; a background job refreshes it every 12h. certbot's own timer renews.

Re-deploy is idempotent: binaries upload atomically (`.new`+rename), and the
port-conflict guard aborts only when a port is held by a process other than our
own frps unit.

## Ports (per server, configurable)

| Port | Role |
|------|------|
| 9000 | frps `bindPort` — frpc control, public |
| 9001 | frps dashboard (`webServer`, 127.0.0.1; nginx :443 proxies here) |
| 9002 | frps `vhostHTTPPort` |
| 9003 | fp-multiuser plugin (127.0.0.1, internal) |

## Troubleshooting

- **`ss`/facts return empty:** the programmatic SSH session uses a minimal PATH;
  `ss` calls are prefixed with `PATH=/usr/sbin:/sbin:$PATH` for this reason.
- **certbot fails HTTP-01:** ensure the domain's A record routes to the VPS
  (the target VPS may have both a public and a management IP on the same box).
  Use the `--staging` path first if testing to avoid LE rate limits.
- **Host key mismatch:** the pinned key changed — investigate before clearing
  `servers.host_key`; it is a possible MITM signal.
- **fp-multiuser not active after add:** check `journalctl -u fp-multiuser-<proj>`;
  a malformed `tokens.txt` line is the usual cause.

## Cloudflare (automatic wildcard cert + DNS) — recommended

If the server's domain is in a Cloudflare zone, the tool automates everything.
Put the Cloudflare **API token** in `data/cloudflare.token` (gitignored, mode
0600) or env `FDM_CLOUDFLARE_TOKEN`. The token needs Zone:DNS:Edit on the zone.

On the server detail page → HTTPS certificate → Cloudflare box:
- **Point DNS → VPS**: upserts A records `<domain>` and `*.<domain>` at the VPS
  public IP (DNS-only / grey cloud — required for non-HTTP frps ports).
- **Issue wildcard cert**: installs `certbot-dns-cloudflare` on the VPS, writes
  the credentials ini (0600), issues a DNS-01 cert for `<domain>` + `*.<domain>`
  (**auto-renews** via certbot's timer), then installs an nginx block:
  `<domain>` → dashboard (9001), `*.<domain>` → frps vhost (9002) for frpc
  subdomain tunnels. frps is configured with `subdomain_host = <domain>`, so a
  frpc registering `subdomain = foo` is served at `foo.<domain>` over HTTPS.

Example verified live: `a.vnmap3d.io.vn` (dashboard) + `*.a.vnmap3d.io.vn`
(frpc tunnels), single wildcard cert, auto-renew.

## frps config format

frps is configured with **legacy INI** (`conf/frps.ini`, `[common]` +
`[plugin.multiuser]`). systemd runs `frps -c .../conf/frps.ini`.

## Manual wildcard DNS-01 (only if DNS is NOT on Cloudflare)

DNS for `vnmap3d.com` is at vdconline.vn (no API), so the wildcard cert is issued
**manually** via DNS-01 and **must be renewed manually** (~every 60 days). It is a
separate cert from the HTTP-01 `io.vnmap3d.com` dashboard cert.

To issue/renew, run on the VPS and add the printed `_acme-challenge.io.vnmap3d.com`
TXT record in the vdconline panel, wait for propagation, then press Enter:

```bash
certbot certonly --manual --preferred-challenges dns \
  -d '*.io.vnmap3d.com' --cert-name io-wildcard \
  --agree-tos -m kspprovn@gmail.com --no-eff-email
# verify TXT first:  dig +short TXT _acme-challenge.io.vnmap3d.com @ns1.vdconline.vn
```

Cert lives at `/etc/letsencrypt/live/io-wildcard/`. For automatic wildcard renewal,
move/delegate `io.vnmap3d.com` DNS to a provider with an API (e.g. Cloudflare) and
switch to `certbot --dns-cloudflare`.

To route frpc subdomain tunnels over HTTPS with this cert, add an nginx vhost
`server_name *.io.vnmap3d.com;` (ssl_certificate → `io-wildcard`) proxying to the
frps `vhostHTTPPort` (9002), and set `subDomainHost = "io.vnmap3d.com"` in
`frps.toml`.

## Known follow-ups

- Deploy currently runs synchronously (~45s); a job + SSE live-log stream is a
  planned enhancement (M11 in the plan).
- Dashboard password is stored plaintext in `servers.dash_password` (it is also
  written to the remote `frps.toml`); consider sealing it at rest.
