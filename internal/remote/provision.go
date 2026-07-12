package remote

import (
	"bytes"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"fmt"
	"sort"
	"text/template"
	"time"

	"github.com/ngohuynhngockhanh/frps-deploy-manager/internal/builder"
	"github.com/ngohuynhngockhanh/frps-deploy-manager/internal/crypto"
	"github.com/ngohuynhngockhanh/frps-deploy-manager/internal/ssh"
	"github.com/ngohuynhngockhanh/frps-deploy-manager/internal/store"
)

//go:embed templates/*.tmpl
var tmplFS embed.FS

var tmpls = template.Must(template.ParseFS(tmplFS, "templates/*.tmpl"))

// LogFn receives progress updates during provisioning.
type LogFn func(phase string, ok bool, detail string)

// Deployer runs the provisioning pipeline against a server.
type Deployer struct {
	Store   *store.Store
	Builder *builder.Builder
	Sealer  *crypto.Sealer
	CFToken string // Cloudflare API token (empty if not configured)
}

// Provision deploys/updates frps + fp-multiuser on the server. It is idempotent:
// re-running re-applies binaries, config and units and restarts the services.
// (nginx + certbot are handled separately in the cert milestone.)
func (d *Deployer) Provision(s store.Server, log LogFn) (store.Server, error) {
	step := func(phase string, ok bool, detail string) {
		_ = d.Store.AddDeployEvent(s.ID, phase, ok, detail)
		if log != nil {
			log(phase, ok, detail)
		}
	}
	fail := func(phase string, err error) (store.Server, error) {
		step(phase, false, err.Error())
		s.Status = "error"
		_ = d.Store.UpdateServer(s)
		return s, fmt.Errorf("%s: %w", phase, err)
	}

	// 1) Connect (+ TOFU pin).
	client, err := ssh.Dial(SSHConfig(s))
	if err != nil {
		return fail("connect", err)
	}
	defer client.Close()
	if client.PinnedOnDial && s.HostKey == "" && client.SeenHostKey != "" {
		s.HostKey = client.SeenHostKey
		_ = d.Store.UpdateServer(s)
	}
	step("connect", true, "host key "+client.Fingerprint())

	// 2) Facts + port-conflict guard (only on a first deploy).
	facts, err := client.Gather(InterestingPorts(s))
	if err != nil {
		return fail("facts", err)
	}
	if !facts.HasSystemd {
		return fail("facts", fmt.Errorf("systemd not available on remote"))
	}
	// Port-conflict guard: abort only when a port is held by something that is
	// NOT our own frps unit (i.e. a genuine conflict on a fresh host). A
	// re-deploy where our frps already holds the ports is fine.
	oursActive := client.Out(fmt.Sprintf("systemctl is-active frps-%s 2>/dev/null", s.Project)) == "active"
	if !oursActive && len(facts.PortsInUse) > 0 {
		return fail("port-check", fmt.Errorf("ports already in use by another process: %v", portList(facts)))
	}
	step("facts", true, facts.Uname)

	// 3) Directories.
	base := s.RemotePath()
	for _, dir := range []string{base + "/bin", base + "/conf"} {
		if err := client.MkdirAll(dir); err != nil {
			return fail("mkdir", err)
		}
	}
	step("mkdir", true, base)

	// 4) Build (cached).
	frpsArt, err := d.Builder.BuildFrps(false)
	if err != nil {
		return fail("build-frps", err)
	}
	fpmuArt, err := d.Builder.BuildFpMultiuser(false)
	if err != nil {
		return fail("build-fpmultiuser", err)
	}
	step("build", true, fmt.Sprintf("frps %s, fp-multiuser %s", frpsArt.Version, fpmuArt.Version))

	// 5) Upload binaries (atomic).
	if err := client.UploadFile(frpsArt.Path, base+"/bin/frps", 0o755); err != nil {
		return fail("upload-frps", err)
	}
	if err := client.UploadFile(fpmuArt.Path, base+"/bin/fp-multiuser", 0o755); err != nil {
		return fail("upload-fpmultiuser", err)
	}
	step("upload", true, "binaries uploaded")

	// 6) tokens.txt from the DB (may be empty until users are added).
	tokens, err := d.renderTokens(s.ID)
	if err != nil {
		return fail("tokens", err)
	}
	if err := client.WriteFile(base+"/conf/tokens.txt", tokens, 0o600); err != nil {
		return fail("tokens", err)
	}
	step("tokens", true, fmt.Sprintf("%d user(s)", countLines(tokens)))

	// 7) frps.toml (generate a dashboard password if unset).
	if s.DashPassword == "" {
		s.DashPassword = randHex(9)
		_ = d.Store.UpdateServer(s)
	}
	ini, err := render("frps.ini.tmpl", s)
	if err != nil {
		return fail("frps.ini", err)
	}
	if err := client.WriteFile(base+"/conf/frps.ini", ini, 0o600); err != nil {
		return fail("frps.ini", err)
	}
	step("frps.ini", true, fmt.Sprintf("dashboard on 127.0.0.1:%d", s.DashboardPort))

	// 8) systemd units.
	fpmuUnit, err := render("fp-multiuser.service.tmpl", s)
	if err != nil {
		return fail("units", err)
	}
	frpsUnit, err := render("frps.service.tmpl", s)
	if err != nil {
		return fail("units", err)
	}
	if err := client.WriteFile(fmt.Sprintf("/etc/systemd/system/fp-multiuser-%s.service", s.Project), fpmuUnit, 0o644); err != nil {
		return fail("units", err)
	}
	if err := client.WriteFile(fmt.Sprintf("/etc/systemd/system/frps-%s.service", s.Project), frpsUnit, 0o644); err != nil {
		return fail("units", err)
	}
	if _, err := client.MustRun("systemctl daemon-reload"); err != nil {
		return fail("daemon-reload", err)
	}
	step("units", true, "systemd units installed")

	// 9) Start fp-multiuser first, then frps.
	if _, err := client.MustRun(fmt.Sprintf("systemctl enable --now fp-multiuser-%s", s.Project)); err != nil {
		return fail("start-fpmultiuser", err)
	}
	if _, err := client.MustRun(fmt.Sprintf("systemctl enable %s && systemctl restart %s",
		"frps-"+s.Project, "frps-"+s.Project)); err != nil {
		return fail("start-frps", err)
	}
	step("start", true, "services started")

	// 10) Verify active + listening.
	if out := client.Out(fmt.Sprintf("systemctl is-active fp-multiuser-%s", s.Project)); out != "active" {
		return fail("verify", fmt.Errorf("fp-multiuser not active (%s): %s", out,
			client.Out(fmt.Sprintf("systemctl status --no-pager -l fp-multiuser-%s | tail -5", s.Project))))
	}
	if out := client.Out(fmt.Sprintf("systemctl is-active frps-%s", s.Project)); out != "active" {
		return fail("verify", fmt.Errorf("frps not active (%s): %s", out,
			client.Out(fmt.Sprintf("systemctl status --no-pager -l frps-%s | tail -5", s.Project))))
	}
	listen := client.Out(fmt.Sprintf("PATH=/usr/sbin:/sbin:$PATH ss -ltnH | awk '{print $4}' | grep -E ':%d$' | head -1", s.BindPort))
	step("verify", true, fmt.Sprintf("both active; bindPort %d %s", s.BindPort,
		yesNo(listen != "", "listening", "NOT listening")))

	// 11) Mark ready.
	s.FrpsVersion = frpsArt.Version
	s.Status = "ready"
	s.LastDeployedAt = time.Now().Unix()
	if err := d.Store.UpdateServer(s); err != nil {
		return fail("finalize", err)
	}
	step("done", true, "deploy complete")
	return s, nil
}

// renderTokens builds the tokens.txt body ("user=token" lines) from the DB.
func (d *Deployer) renderTokens(serverID int64) ([]byte, error) {
	blobs, err := d.Store.FrpTokenBlobs(serverID)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(blobs))
	for n := range blobs {
		names = append(names, n)
	}
	sort.Strings(names)
	var buf bytes.Buffer
	for _, n := range names {
		tok, err := d.Sealer.Open(blobs[n])
		if err != nil {
			return nil, fmt.Errorf("decrypt token for %s: %w", n, err)
		}
		fmt.Fprintf(&buf, "%s=%s\n", n, tok)
	}
	return buf.Bytes(), nil
}

func render(name string, data any) ([]byte, error) {
	var buf bytes.Buffer
	if err := tmpls.ExecuteTemplate(&buf, name, data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func portList(f ssh.Facts) []int {
	var ps []int
	for p := range f.PortsInUse {
		ps = append(ps, p)
	}
	sort.Ints(ps)
	return ps
}

func countLines(b []byte) int {
	if len(b) == 0 {
		return 0
	}
	return bytes.Count(b, []byte("\n"))
}

func yesNo(cond bool, y, n string) string {
	if cond {
		return y
	}
	return n
}
