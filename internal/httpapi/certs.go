package httpapi

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/ngohuynhngockhanh/frps-deploy-manager/internal/auth"
	"github.com/ngohuynhngockhanh/frps-deploy-manager/internal/store"
)

type certView struct {
	Server  store.Server
	Cert    *store.Cert
	HasCert bool
	HasCF   bool // a Cloudflare token is configured
	Err     string
	Msg     string
	CSRF    string
}

// certPanel renders the certificate panel (GET, htmx-loaded on the detail page).
func (d Deps) certPanel(w http.ResponseWriter, r *http.Request) {
	s, ok := d.lookupServer(w, r)
	if !ok {
		return
	}
	d.renderCert(w, r, s, "", "")
}

// certIssue installs the nginx block and obtains/renews the certificate.
func (d Deps) certIssue(w http.ResponseWriter, r *http.Request) {
	s, ok := d.lookupServer(w, r)
	if !ok {
		return
	}
	staging := r.PostFormValue("staging") != ""
	if _, err := d.Deployer.IssueCert(s, staging, nil); err != nil {
		d.renderCert(w, r, s, err.Error(), "")
		return
	}
	d.renderCert(w, r, s, "", "")
}

// certRefresh re-reads the cert to update expiry.
func (d Deps) certRefresh(w http.ResponseWriter, r *http.Request) {
	s, ok := d.lookupServer(w, r)
	if !ok {
		return
	}
	if _, err := d.Deployer.RefreshCert(s); err != nil {
		d.renderCert(w, r, s, err.Error(), "")
		return
	}
	d.renderCert(w, r, s, "", "")
}

// certPointDNS upserts Cloudflare A records for <domain> and *.<domain>.
func (d Deps) certPointDNS(w http.ResponseWriter, r *http.Request) {
	s, ok := d.lookupServer(w, r)
	if !ok {
		return
	}
	ip, err := d.Deployer.PointDNS(s)
	if err != nil {
		d.renderCert(w, r, s, err.Error(), "")
		return
	}
	d.renderCert(w, r, s, "", fmt.Sprintf("DNS pointed: %s and *.%s → %s (Cloudflare, DNS-only).", s.Domain, s.Domain, ip))
}

// certIssueCloudflare issues a wildcard cert via DNS-01 (certbot dns-cloudflare)
// and installs the base+wildcard nginx block.
func (d Deps) certIssueCloudflare(w http.ResponseWriter, r *http.Request) {
	s, ok := d.lookupServer(w, r)
	if !ok {
		return
	}
	if _, err := d.Deployer.IssueCertCloudflare(s, nil); err != nil {
		d.renderCert(w, r, s, err.Error(), "")
		return
	}
	d.renderCert(w, r, s, "", "Wildcard cert issued for *."+s.Domain+" (auto-renews via certbot).")
}

func (d Deps) renderCert(w http.ResponseWriter, r *http.Request, s store.Server, errMsg, okMsg string) {
	view := certView{Server: s, Err: errMsg, Msg: okMsg, CSRF: auth.CSRFToken(r), HasCF: d.Deployer.CFToken != ""}
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
