package httpapi

import (
	"errors"
	"net/http"

	"github.com/ngohuynhngockhanh/frps-deploy-manager/internal/auth"
	"github.com/ngohuynhngockhanh/frps-deploy-manager/internal/store"
)

type certView struct {
	Server  store.Server
	Cert    *store.Cert
	HasCert bool
	Err     string
	CSRF    string
}

// certPanel renders the certificate panel (GET, htmx-loaded on the detail page).
func (d Deps) certPanel(w http.ResponseWriter, r *http.Request) {
	s, ok := d.lookupServer(w, r)
	if !ok {
		return
	}
	d.renderCert(w, r, s, "")
}

// certIssue installs the nginx block and obtains/renews the certificate.
func (d Deps) certIssue(w http.ResponseWriter, r *http.Request) {
	s, ok := d.lookupServer(w, r)
	if !ok {
		return
	}
	staging := r.PostFormValue("staging") != ""
	if _, err := d.Deployer.IssueCert(s, staging, nil); err != nil {
		d.renderCert(w, r, s, err.Error())
		return
	}
	d.renderCert(w, r, s, "")
}

// certRefresh re-reads the cert to update expiry.
func (d Deps) certRefresh(w http.ResponseWriter, r *http.Request) {
	s, ok := d.lookupServer(w, r)
	if !ok {
		return
	}
	if _, err := d.Deployer.RefreshCert(s); err != nil {
		d.renderCert(w, r, s, err.Error())
		return
	}
	d.renderCert(w, r, s, "")
}

func (d Deps) renderCert(w http.ResponseWriter, r *http.Request, s store.Server, errMsg string) {
	view := certView{Server: s, Err: errMsg, CSRF: auth.CSRFToken(r)}
	if s.Domain != "" {
		if c, err := d.Store.GetCert(s.ID, s.Domain); err == nil {
			view.Cert = &c
			view.HasCert = true
		} else if !errors.Is(err, store.ErrNotFound) {
			view.Err = err.Error()
		}
	}
	if err := d.Render.RenderPartial(w, "_cert.html", view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
