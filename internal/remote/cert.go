package remote

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"

	"github.com/ngohuynhngockhanh/frps-deploy-manager/internal/ssh"
	"github.com/ngohuynhngockhanh/frps-deploy-manager/internal/store"
)

// IssueCert installs the nginx server block for the server's domain, obtains (or
// renews) a Let's Encrypt certificate via certbot --nginx, then reads the cert
// back and records its metadata. staging uses LE's staging CA for testing.
func (d *Deployer) IssueCert(s store.Server, staging bool, log LogFn) (store.Cert, error) {
	step := func(phase string, ok bool, detail string) {
		_ = d.Store.AddDeployEvent(s.ID, phase, ok, detail)
		if log != nil {
			log(phase, ok, detail)
		}
	}
	if s.Domain == "" {
		return store.Cert{}, fmt.Errorf("server has no domain configured")
	}
	if s.CertEmail == "" {
		return store.Cert{}, fmt.Errorf("server has no cert email configured")
	}

	client, err := ssh.Dial(SSHConfig(s))
	if err != nil {
		return store.Cert{}, err
	}
	defer client.Close()

	if client.Out("command -v certbot") == "" {
		return store.Cert{}, fmt.Errorf("certbot not installed on remote")
	}

	// 1) nginx server block.
	conf, err := render("nginx.conf.tmpl", s)
	if err != nil {
		return store.Cert{}, err
	}
	avail := "/etc/nginx/sites-available/" + s.Domain + ".conf"
	enabled := "/etc/nginx/sites-enabled/" + s.Domain + ".conf"
	if err := client.WriteFile(avail, conf, 0o644); err != nil {
		return store.Cert{}, fmt.Errorf("write nginx conf: %w", err)
	}
	if _, err := client.MustRun(fmt.Sprintf("ln -sf %s %s", avail, enabled)); err != nil {
		return store.Cert{}, err
	}
	if out, err := client.MustRun("nginx -t 2>&1"); err != nil {
		return store.Cert{}, fmt.Errorf("nginx config test failed: %s", out)
	}
	if _, err := client.MustRun("systemctl reload nginx"); err != nil {
		return store.Cert{}, err
	}
	step("nginx", true, "server block for "+s.Domain+" installed")

	// 2) certbot issuance (idempotent; won't reissue a still-valid cert).
	stagingFlag := ""
	if staging {
		stagingFlag = " --staging"
	}
	cmd := fmt.Sprintf("certbot --nginx -d %s --non-interactive --agree-tos -m %s --redirect --keep-until-expiring%s 2>&1",
		s.Domain, s.CertEmail, stagingFlag)
	out, err := client.Run(cmd)
	if err != nil {
		return store.Cert{}, fmt.Errorf("run certbot: %w", err)
	}
	if out.Exit != 0 {
		return store.Cert{}, fmt.Errorf("certbot failed: %s", tail(out.Stdout+out.Stderr, 600))
	}
	step("certbot", true, "certificate obtained/renewed")

	// 3) Read the cert and record metadata.
	c, err := d.refreshCertFromRemote(client, s)
	if err != nil {
		return store.Cert{}, err
	}
	step("cert", true, fmt.Sprintf("expires in %d days", c.DaysRemaining()))
	return c, nil
}

// refreshCertFromRemote fetches the live cert PEM over sftp, parses it and
// upserts the record.
func (d *Deployer) refreshCertFromRemote(client *ssh.Client, s store.Server) (store.Cert, error) {
	certPath := "/etc/letsencrypt/live/" + s.Domain + "/cert.pem"
	pemBytes, err := client.ReadFile(certPath)
	if err != nil {
		return store.Cert{}, fmt.Errorf("read cert: %w", err)
	}
	c, err := parseCert(pemBytes)
	if err != nil {
		return store.Cert{}, err
	}
	c.ServerID = s.ID
	c.Domain = s.Domain
	c.CertPath = certPath
	if err := d.Store.UpsertCert(c); err != nil {
		return store.Cert{}, err
	}
	return c, nil
}

// RefreshCert reconnects and re-reads the cert for expiry tracking.
func (d *Deployer) RefreshCert(s store.Server) (store.Cert, error) {
	client, err := ssh.Dial(SSHConfig(s))
	if err != nil {
		return store.Cert{}, err
	}
	defer client.Close()
	return d.refreshCertFromRemote(client, s)
}

// parseCert extracts metadata from a PEM certificate.
func parseCert(pemBytes []byte) (store.Cert, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return store.Cert{}, fmt.Errorf("no PEM block in certificate")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return store.Cert{}, err
	}
	return store.Cert{
		Issuer:    cert.Issuer.CommonName,
		NotBefore: cert.NotBefore.Unix(),
		NotAfter:  cert.NotAfter.Unix(),
		Serial:    cert.SerialNumber.Text(16),
	}, nil
}

func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return "…" + s[len(s)-n:]
}
