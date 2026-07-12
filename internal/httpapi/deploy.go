package httpapi

import (
	"net/http"

	"github.com/ngohuynhngockhanh/frps-deploy-manager/internal/store"
)

type deployStep struct {
	Phase  string
	OK     bool
	Detail string
}

type deployResult struct {
	Steps  []deployStep
	Err    string
	Server store.Server
}

// deploy runs the provisioning pipeline synchronously and returns an htmx
// partial with the per-phase log. (M11 converts this to a streamed job.)
func (d Deps) deploy(w http.ResponseWriter, r *http.Request) {
	s, ok := d.lookupServer(w, r)
	if !ok {
		return
	}

	var steps []deployStep
	logf := func(phase string, ok bool, detail string) {
		steps = append(steps, deployStep{Phase: phase, OK: ok, Detail: detail})
	}

	updated, err := d.Deployer.Provision(s, logf)
	res := deployResult{Steps: steps, Server: updated}
	if err != nil {
		res.Err = err.Error()
	}
	_ = d.Render.RenderPartial(w, "_deploy.html", res)
}
