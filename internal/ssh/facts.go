package ssh

import (
	"fmt"
	"strings"
)

// Facts summarises the remote host's relevant capabilities and port usage.
type Facts struct {
	Uname      string         // "Linux 6.x x86_64"
	Arch       string         // "x86_64"
	HasNginx   bool
	HasCertbot bool
	HasSystemd bool
	NginxVer   string
	CertbotVer string
	PortsInUse map[int]string // port -> listening process/desc (from ss)
}

// Gather collects facts and checks whether any of the given ports are in use.
func (c *Client) Gather(ports []int) (Facts, error) {
	f := Facts{PortsInUse: map[int]string{}}

	uname := c.Out("uname -s -r -m")
	if uname == "" {
		// A totally empty uname means the session/exec failed.
		if _, err := c.Run("true"); err != nil {
			return f, fmt.Errorf("remote exec failed: %w", err)
		}
	}
	f.Uname = uname
	if fields := strings.Fields(uname); len(fields) > 0 {
		f.Arch = fields[len(fields)-1]
	}

	f.HasNginx = c.Out("command -v nginx") != ""
	f.HasCertbot = c.Out("command -v certbot") != ""
	f.HasSystemd = c.Out("command -v systemctl") != ""
	if f.HasNginx {
		f.NginxVer = c.Out("nginx -v 2>&1")
	}
	if f.HasCertbot {
		f.CertbotVer = c.Out("certbot --version 2>&1")
	}

	// Listening TCP ports. `ss -ltnH` prints one row per socket; column 4 is the
	// local address:port. Match our ports of interest.
	want := map[int]bool{}
	for _, p := range ports {
		want[p] = true
	}
	out := c.Out("ss -ltnH 2>/dev/null || ss -ltn 2>/dev/null")
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		local := fields[3]
		p := portOf(local)
		if p != 0 && want[p] {
			f.PortsInUse[p] = strings.TrimSpace(line)
		}
	}
	return f, nil
}

func portOf(hostport string) int {
	i := strings.LastIndexByte(hostport, ':')
	if i < 0 {
		return 0
	}
	var p int
	if _, err := fmt.Sscanf(hostport[i+1:], "%d", &p); err != nil {
		return 0
	}
	return p
}
