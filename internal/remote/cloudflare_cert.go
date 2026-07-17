package remote

import (
	"fmt"
	"strings"

	"github.com/ngohuynhngockhanh/frps-deploy-manager/internal/cloudflare"
	"github.com/ngohuynhngockhanh/frps-deploy-manager/internal/ssh"
	"github.com/ngohuynhngockhanh/frps-deploy-manager/internal/store"
)

// PointDNS upserts A records for <domain> and *.<domain> in Cloudflare, both
// pointing at the VPS public IP (DNS-only, not proxied — frps control/vhost
// ports are non-HTTP and must reach the VPS directly). Returns the IP used.
// cfToken returns the server's own Cloudflare token if set, else the global one.
func (d *Deployer) cfToken(s store.Server) string {
	if s.CFToken != "" {
		return s.CFToken
	}
	return d.CFToken
}

func (d *Deployer) PointDNS(s store.Server) (string, error) {
	tok := d.cfToken(s)
	if tok == "" {
		return "", fmt.Errorf("no Cloudflare token configured")
	}
	if s.Domain == "" {
		return "", fmt.Errorf("server has no domain")
	}
	client, err := ssh.Dial(SSHConfig(s))
	if err != nil {
		return "", err
	}
	ip := client.Out("curl -s --max-time 8 https://api.ipify.org")
	client.Close()
	if !looksLikeIPv4(ip) {
		return "", fmt.Errorf("could not determine VPS public IP (got %q)", ip)
	}

	cf := cloudflare.New(tok)
	zoneID, _, err := cf.ZoneID(s.Domain)
	if err != nil {
		return "", err
	}
	if err := cf.UpsertA(zoneID, s.Domain, ip, false); err != nil {
		return "", fmt.Errorf("A %s: %w", s.Domain, err)
	}
	if err := cf.UpsertA(zoneID, "*."+s.Domain, ip, false); err != nil {
		return "", fmt.Errorf("A *.%s: %w", s.Domain, err)
	}
	return ip, nil
}

// IssueCertCloudflare obtains a wildcard cert (<domain> + *.<domain>) via
// DNS-01 using certbot's dns-cloudflare plugin (auto-renewable), then installs
// an nginx block routing the base domain to the dashboard and *.<domain> to the
// frps vhost, and records the cert.
func (d *Deployer) IssueCertCloudflare(s store.Server, actor string, log LogFn) (store.Cert, error) {
	step := func(phase string, ok bool, detail string) {
		_ = d.Store.AddDeployEvent(s.ID, actor, phase, ok, detail)
		if log != nil {
			log(phase, ok, detail)
		}
	}
	tok := d.cfToken(s)
	if tok == "" {
		return store.Cert{}, fmt.Errorf("no Cloudflare token configured")
	}
	if s.Domain == "" || s.CertEmail == "" {
		return store.Cert{}, fmt.Errorf("server needs a domain and cert email")
	}

	client, err := ssh.Dial(SSHConfig(s))
	if err != nil {
		return store.Cert{}, err
	}
	defer client.Close()

	// 1) Ensure the dns-cloudflare plugin is available (certbot is a pip venv).
	if client.Out("certbot plugins 2>/dev/null | grep -c dns-cloudflare") == "0" {
		pip := client.Out("command -v /opt/certbot/bin/pip || command -v pip3 || command -v pip")
		if pip == "" {
			return store.Cert{}, fmt.Errorf("cannot find pip to install certbot-dns-cloudflare")
		}
		if out, err := client.MustRun(pip + " install --quiet certbot-dns-cloudflare 2>&1"); err != nil {
			return store.Cert{}, fmt.Errorf("install plugin: %s", tail(out, 400))
		}
	}
	step("cf-plugin", true, "certbot dns-cloudflare ready")

	// 2) Credentials ini at a STABLE path (not the deploy dir, which can be
	//    renamed) so certbot's auto-renew keeps working. 0600.
	ini := "/etc/letsencrypt/cloudflare-" + s.Domain + ".ini"
	if err := client.WriteFile(ini, []byte("dns_cloudflare_api_token = "+tok+"\n"), 0o600); err != nil {
		return store.Cert{}, err
	}

	// 3) Issue wildcard cert (idempotent).
	cmd := fmt.Sprintf("certbot certonly --dns-cloudflare --dns-cloudflare-credentials %s "+
		"--dns-cloudflare-propagation-seconds 30 -d %s -d '*.%s' --cert-name %s "+
		"--agree-tos -m %s --non-interactive --keep-until-expiring 2>&1",
		ini, s.Domain, s.Domain, s.Domain, s.CertEmail)
	out, err := client.Run(cmd)
	if err != nil {
		return store.Cert{}, fmt.Errorf("run certbot: %w", err)
	}
	if out.Exit != 0 {
		return store.Cert{}, fmt.Errorf("certbot failed: %s", tail(out.Stdout+out.Stderr, 700))
	}
	step("certbot", true, "wildcard cert obtained for *."+s.Domain)

	// 4) nginx block (base -> dashboard, wildcard -> vhost) with the cert.
	conf, err := render("nginx-wildcard.conf.tmpl", s)
	if err != nil {
		return store.Cert{}, err
	}
	// Provide $connection_upgrade map once (idempotent include file).
	if err := client.WriteFile("/etc/nginx/conf.d/fdm-upgrade.conf",
		[]byte("map $http_upgrade $connection_upgrade { default upgrade; '' close; }\n"), 0o644); err != nil {
		return store.Cert{}, err
	}
	avail := "/etc/nginx/sites-available/" + s.Domain + ".conf"
	enabled := "/etc/nginx/sites-enabled/" + s.Domain + ".conf"
	if err := client.WriteFile(avail, conf, 0o644); err != nil {
		return store.Cert{}, err
	}
	if _, err := client.MustRun(fmt.Sprintf("ln -sf %s %s", avail, enabled)); err != nil {
		return store.Cert{}, err
	}
	if out, err := client.MustRun("nginx -t 2>&1"); err != nil {
		return store.Cert{}, fmt.Errorf("nginx test failed: %s", out)
	}
	if _, err := client.MustRun("systemctl reload nginx"); err != nil {
		return store.Cert{}, err
	}
	step("nginx", true, "base->dashboard, *."+s.Domain+"->vhost")

	// 5) Record cert metadata.
	c, err := d.refreshCertFromRemote(client, s)
	if err != nil {
		return store.Cert{}, err
	}
	step("cert", true, fmt.Sprintf("expires in %d days", c.DaysRemaining()))
	return c, nil
}

func looksLikeIPv4(s string) bool {
	s = strings.TrimSpace(s)
	parts := strings.Split(s, ".")
	if len(parts) != 4 {
		return false
	}
	for _, p := range parts {
		if p == "" || len(p) > 3 {
			return false
		}
		for _, r := range p {
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	return true
}
