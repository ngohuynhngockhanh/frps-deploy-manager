package httpapi

import (
	"net/http"

	"github.com/ngohuynhngockhanh/frps-deploy-manager/internal/remote"
	"github.com/ngohuynhngockhanh/frps-deploy-manager/internal/ssh"
)

type portConflict struct {
	Port   int
	Role   string
	Detail string
	Own    bool // held by this server's own frps (not a real conflict)
}

type sshTestResult struct {
	OK          bool
	Err         string
	Fingerprint string
	NewlyPinned bool
	Facts       ssh.Facts
	Conflicts   []portConflict
}

// testSSH connects to the server, pins the host key on first sight (TOFU),
// gathers remote facts and reports any port conflicts. Returns an htmx partial.
func (d Deps) testSSH(w http.ResponseWriter, r *http.Request) {
	s, ok := d.lookupServer(w, r)
	if !ok {
		return
	}

	client, err := ssh.Dial(remote.SSHConfig(s))
	if err != nil {
		_ = d.Render.RenderPartial(w, "_testssh.html", sshTestResult{Err: err.Error()})
		return
	}
	defer client.Close()

	res := sshTestResult{OK: true, Fingerprint: client.Fingerprint()}

	// TOFU: persist a newly-seen host key.
	if client.PinnedOnDial && s.HostKey == "" && client.SeenHostKey != "" {
		s.HostKey = client.SeenHostKey
		if err := d.Store.UpdateServer(s); err == nil {
			res.NewlyPinned = true
		}
	}

	// Ports held by this server's own (already-deployed) frps are not conflicts.
	ownFrps := client.Out("systemctl is-active frps-"+s.Project) == "active"

	facts, err := client.Gather(remote.InterestingPorts(s))
	if err != nil {
		res.OK = false
		res.Err = err.Error()
		_ = d.Render.RenderPartial(w, "_testssh.html", res)
		return
	}
	res.Facts = facts
	for port, detail := range facts.PortsInUse {
		res.Conflicts = append(res.Conflicts, portConflict{
			Port: port, Role: remote.PortRole(s, port), Detail: detail, Own: ownFrps,
		})
	}
	_ = d.Render.RenderPartial(w, "_testssh.html", res)
}
