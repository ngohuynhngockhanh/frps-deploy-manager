package httpapi

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/ngohuynhngockhanh/frps-deploy-manager/internal/auth"
	"github.com/ngohuynhngockhanh/frps-deploy-manager/internal/store"
)

// certDomain is one row in the cert panel: a domain served by this server and
// its current certificate state.
type certDomain struct {
	Domain  string
	Cert    *store.Cert
	HasCert bool
	HasCF   bool // a Cloudflare token is configured for this domain's zone
	Primary bool
}

type certView struct {
	Server  store.Server
	Domains []certDomain
	IsAdmin bool
	Err     string
	Msg     string
	CSRF    string
}

// resolveDomainToken maps the posted "domain" value to one of the server's
// domains and the Cloudflare token to use for it. Empty defaults to primary.
func (d Deps) resolveDomainToken(s store.Server, param string) (domain, token string, ok bool) {
	if param == "" || param == s.Domain {
		tok := s.CFToken
		if tok == "" {
			tok = d.Deployer.CFToken
		}
		return s.Domain, tok, s.Domain != ""
	}
	if param == s.Domain2 && s.Domain2 != "" {
		tok := s.CFToken2
		if tok == "" {
			tok = d.Deployer.CFToken
		}
		return s.Domain2, tok, true
	}
	return "", "", false
}

// certPanel renders the certificate panel (GET, htmx-loaded on the detail page).
func (d Deps) certPanel(w http.ResponseWriter, r *http.Request) {
	s, ok := d.lookupServer(w, r)
	if !ok {
		return
	}
	d.renderCert(w, r, s, "", "")
}

// certIssue installs the nginx block and obtains/renews the certificate (HTTP-01,
// primary domain only — used when no Cloudflare token is configured).
func (d Deps) certIssue(w http.ResponseWriter, r *http.Request) {
	s, ok := d.lookupServer(w, r)
	if !ok {
		return
	}
	staging := r.PostFormValue("staging") != ""
	if _, err := d.Deployer.IssueCert(s, staging, auth.CurrentUser(r), nil); err != nil {
		d.renderCert(w, r, s, err.Error(), "")
		return
	}
	d.renderCert(w, r, s, "", "")
}

// certRefresh re-reads the cert to update expiry for the posted domain.
func (d Deps) certRefresh(w http.ResponseWriter, r *http.Request) {
	s, ok := d.lookupServer(w, r)
	if !ok {
		return
	}
	domain, _, valid := d.resolveDomainToken(s, r.PostFormValue("domain"))
	if !valid {
		d.renderCert(w, r, s, "unknown domain", "")
		return
	}
	if _, err := d.Deployer.RefreshCertDomain(s, domain); err != nil {
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
	domain, token, valid := d.resolveDomainToken(s, r.PostFormValue("domain"))
	if !valid {
		d.renderCert(w, r, s, "unknown domain", "")
		return
	}
	ip, err := d.Deployer.PointDNSFor(s, domain, token)
	if err != nil {
		d.renderCert(w, r, s, err.Error(), "")
		return
	}
	d.renderCert(w, r, s, "", fmt.Sprintf("DNS pointed: %s and *.%s → %s (Cloudflare, DNS-only).", domain, domain, ip))
}

// certIssueCloudflare issues a wildcard cert via DNS-01 (certbot dns-cloudflare)
// and installs the base+wildcard nginx block for the posted domain.
func (d Deps) certIssueCloudflare(w http.ResponseWriter, r *http.Request) {
	s, ok := d.lookupServer(w, r)
	if !ok {
		return
	}
	domain, token, valid := d.resolveDomainToken(s, r.PostFormValue("domain"))
	if !valid {
		d.renderCert(w, r, s, "unknown domain", "")
		return
	}
	if _, err := d.Deployer.IssueCertCloudflareFor(s, domain, token, auth.CurrentUser(r), nil); err != nil {
		d.renderCert(w, r, s, err.Error(), "")
		return
	}
	d.renderCert(w, r, s, "", "Wildcard cert issued for *."+domain+" (auto-renews via certbot).")
}

func (d Deps) renderCert(w http.ResponseWriter, r *http.Request, s store.Server, errMsg, okMsg string) {
	view := certView{Server: s, Err: errMsg, Msg: okMsg, CSRF: auth.CSRFToken(r), IsAdmin: auth.IsAdmin(r)}

	add := func(domain string, primary bool, hasCF bool) {
		if domain == "" {
			return
		}
		cd := certDomain{Domain: domain, Primary: primary, HasCF: hasCF}
		if c, err := d.Store.GetCert(s.ID, domain); err == nil {
			cc := c
			cd.Cert = &cc
			cd.HasCert = true
		} else if !errors.Is(err, store.ErrNotFound) {
			view.Err = err.Error()
		}
		view.Domains = append(view.Domains, cd)
	}
	add(s.Domain, true, d.Deployer.CFToken != "" || s.CFToken != "")
	add(s.Domain2, false, s.CFToken2 != "" || d.Deployer.CFToken != "")

	if err := d.Render.RenderPartial(w, "_cert.html", view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
