---
name: deploy-frps-over-vpn
description: Deploy and operate FRPS over a VPN management address while publishing services through a public Floating IP, Cloudflare DNS, nginx, and automatic wildcard TLS renewal.
---

# Deploy FRPS Over VPN

Use this workflow when the VPS management address is a VPN/tunnel address but
Internet traffic must enter through a public Floating IP. Keep the address
roles separate:

| Role | Reference | Rule |
| --- | --- | --- |
| VPN/management | `77.88.208.17` | SSH and private health checks only |
| Public Floating IP | `103.226.248.95` | Cloudflare A records and public traffic |

Never point public DNS at the VPN address just because it is reachable from the
operator network. A successful VPN probe does not prove Internet reachability.

## Reference deployment

Server `#4` (`inut-a-frp`) is the reference deployment:

- SSH: `root@77.88.208.17:22` using the existing SSH agent/key.
- Project: `/root/inut-a-frp`.
- Domain: `a.inut.io.vn`; wildcard: `*.a.inut.io.vn`.
- FRPS `bindPort`: `7008`.
- Dashboard: `127.0.0.1:7508` behind nginx HTTPS.
- Vhost HTTP: `8098` behind wildcard nginx HTTPS.
- fp-multiuser: `127.0.0.1:7201`.
- NAT policy: `10000-60000`, maximum `50` ports per client.

Do not copy dashboard passwords, FRP tokens, Cloudflare tokens, private keys,
or session cookies into this skill or Git.

## Preconditions

1. Confirm SSH access and host identity. The manager pins the first-seen host
   key (TOFU); investigate a later mismatch instead of clearing it.
2. Confirm the Floating IP is associated with the target OpenStack port and the
   security group permits `80`, `443`, FRPS `bindPort`, and the vhost port.
   Do not add the Floating IP as a guest address unless the cloud network
   documentation explicitly requires it.
3. Use a least-privilege Cloudflare token with Zone DNS Edit. Keep it in the
   manager's encrypted per-server field or a mode-`0600` secret file.

## Deployment sequence

### 1. Register and test

Create the server in the manager with the VPN host, SSH port, domain, cert
email, and FRPS ports. Use the existing SSH agent unless a dedicated key path
is required. Run **Test SSH & port** and stop on a real port conflict.

### 2. Deploy FRPS

Run **Deploy FRPS**. The manager builds cached Linux/amd64 binaries, uploads
them atomically, writes `frps.ini` and systemd units, starts fp-multiuser before
frps, and verifies the bind port.

```bash
systemctl is-active fp-multiuser-inut-a-frp frps-inut-a-frp
ss -ltnp | grep -E ':(7008|8098)\b'
```

The generated dashboard password is shown once in the deploy result; never
commit or document it.

### 3. Install the HTTPS edge

On a minimal Ubuntu VPS:

```bash
export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get install -y nginx certbot python3-certbot-nginx python3-certbot-dns-cloudflare
```

The base domain proxies to `127.0.0.1:<dashboardPort>` and wildcard traffic
proxies to `vhostHTTPPort`. Keep nginx bound to all interfaces; the cloud
security group is the public exposure boundary.

### 4. Point Cloudflare at the Floating IP

Both records must be DNS-only (grey cloud):

```text
a.inut.io.vn       A 103.226.248.95
*.a.inut.io.vn     A 103.226.248.95
```

DNS-only is required for FRPS control and non-HTTP tunnel ports. The manager's
**Point DNS -> VPS** action derives an address from `api.ipify.org`; review its
result and correct the records if it reports a VPN or wrong NAT address.

```bash
dig +short A a.inut.io.vn @8.8.8.8
dig +short A '*.a.inut.io.vn' @8.8.8.8
```

### 5. Issue wildcard TLS

Use **Issue wildcard cert** in the manager. It writes
`/etc/letsencrypt/cloudflare-a.inut.io.vn.ini` with mode `0600`, issues the base
and wildcard names, installs nginx, and reloads it.

```bash
nginx -t
systemctl is-active nginx certbot.timer
certbot certificates
```

The renewal configuration must reference the stable credentials path, not a
temporary deploy directory.

### 6. Verify the public path

VPN success is not enough; test the Floating IP path:

```bash
curl -I http://a.inut.io.vn/
curl -k -I https://a.inut.io.vn/
openssl s_client -connect a.inut.io.vn:443 -servername a.inut.io.vn \
  </dev/null 2>/dev/null | openssl x509 -noout -dates -ext subjectAltName
```

Expected results: HTTP redirects to HTTPS, HTTPS returns `401` from the
protected dashboard, and the certificate contains both base and wildcard SANs.

## Renewal and monitoring

- Keep `certbot.timer` enabled; DNS-01 renewal does not depend on HTTP reachability.
- Keep the Cloudflare credentials file owned by root and mode `0600`.
- A dry-run uses Let's Encrypt staging and can transiently report
  `authorization must be pending`; inspect `/var/log/letsencrypt/letsencrypt.log`
  and retry after stale staging authorizations clear.
- Check FRPS status through the manager; managers receive sanitized statistics.

## Troubleshooting

### Floating IP resolves but ports time out

If nginx/FRPS listen on `0.0.0.0`, UFW/iptables accept traffic, and the VPN
address works, the failure is upstream of the guest. Check OpenStack:

1. Floating IP `103.226.248.95` is associated with the instance's Neutron port.
2. The security group allows TCP `80`, `443`, `7008`, `8098`, and required NAT ports.
3. No competing instance owns the Floating IP.

Do not add the Floating IP to the guest interface blindly; that can break the
fixed-address route and is not the normal OpenStack model.

### Certificate or DNS failure

- Confirm the Cloudflare token can edit the `inut.io.vn` zone.
- Confirm the credentials path exists and is mode `0600`.
- Remove stale TXT records only after confirming they belong to an abandoned
  challenge; never delete unrelated `_acme-challenge` records.
- Re-run `certbot renew --dry-run --no-random-sleep-on-renew` and inspect the
  complete log if it fails.

### FRPS service failure

```bash
journalctl -u fp-multiuser-inut-a-frp -u frps-inut-a-frp --no-pager -n 100
systemctl restart fp-multiuser-inut-a-frp frps-inut-a-frp
```

Verify `tokens.txt` and port ownership before restarting.

## Security rules

- Use the VPN address for management and the Floating IP for public DNS.
- Keep Cloudflare tokens, dashboard passwords, FRP tokens, SSH keys, and
  `data/master.key` out of Git and logs.
- Preserve host-key pinning and investigate mismatches.
- Keep dashboard and fp-multiuser listeners on loopback; expose only required
  FRPS/vhost/NAT ports.
