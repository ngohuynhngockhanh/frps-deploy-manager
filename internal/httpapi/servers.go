package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/ngohuynhngockhanh/frps-deploy-manager/internal/store"
)

// newServerDefaults returns a Server pre-filled with sensible defaults for the
// "add server" form.
func newServerDefaults() store.Server {
	return store.Server{
		SSHPort: 6464, SSHUser: "root", AuthKind: "agent",
		Project: "frps", RemoteBase: "/root/inut-jsc",
		BindPort: 9000, DashboardPort: 9001, VhostHTTPPort: 9002, FpmuPort: 9003,
		DashUser: "admin",
	}
}

func (d Deps) serverNew(w http.ResponseWriter, r *http.Request) {
	d.render(w, r, "server_form.html", formData{Server: newServerDefaults(), IsNew: true})
}

func (d Deps) serverEdit(w http.ResponseWriter, r *http.Request) {
	s, ok := d.lookupServer(w, r)
	if !ok {
		return
	}
	d.render(w, r, "server_form.html", formData{Server: s, IsNew: false})
}

func (d Deps) serverDetail(w http.ResponseWriter, r *http.Request) {
	s, ok := d.lookupServer(w, r)
	if !ok {
		return
	}
	d.render(w, r, "server_detail.html", s)
}

// serverCreate handles POST /servers.
func (d Deps) serverCreate(w http.ResponseWriter, r *http.Request) {
	s, err := parseServerForm(r, newServerDefaults())
	if err != nil {
		d.render(w, r, "server_form.html", formData{Server: s, IsNew: true, Err: err.Error()})
		return
	}
	created, err := d.Store.CreateServer(s)
	if err != nil {
		d.render(w, r, "server_form.html", formData{Server: s, IsNew: true, Err: friendlyDBErr(err)})
		return
	}
	http.Redirect(w, r, "/servers/"+strconv.FormatInt(created.ID, 10), http.StatusSeeOther)
}

// serverUpdate handles POST /servers/{id}.
func (d Deps) serverUpdate(w http.ResponseWriter, r *http.Request) {
	cur, ok := d.lookupServer(w, r)
	if !ok {
		return
	}
	s, err := parseServerForm(r, cur)
	if err != nil {
		d.render(w, r, "server_form.html", formData{Server: s, IsNew: false, Err: err.Error()})
		return
	}
	s.ID = cur.ID
	if err := d.Store.UpdateServer(s); err != nil {
		d.render(w, r, "server_form.html", formData{Server: s, IsNew: false, Err: friendlyDBErr(err)})
		return
	}
	http.Redirect(w, r, "/servers/"+strconv.FormatInt(cur.ID, 10), http.StatusSeeOther)
}

// serverDelete handles POST /servers/{id}/delete.
func (d Deps) serverDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	if err := d.Store.DeleteServer(id); err != nil && !errors.Is(err, store.ErrNotFound) {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// --- helpers ---

type formData struct {
	Server store.Server
	IsNew  bool
	Err    string
}

func (d Deps) render(w http.ResponseWriter, r *http.Request, page string, data any) {
	if err := d.Render.Render(w, page, d.page(r, data)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (d Deps) lookupServer(w http.ResponseWriter, r *http.Request) (store.Server, bool) {
	id, ok := parseID(w, r)
	if !ok {
		return store.Server{}, false
	}
	s, err := d.Store.GetServer(id)
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return store.Server{}, false
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return store.Server{}, false
	}
	return s, true
}

func parseID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return 0, false
	}
	return id, true
}

func parseServerForm(r *http.Request, base store.Server) (store.Server, error) {
	if err := r.ParseForm(); err != nil {
		return base, err
	}
	s := base
	s.Name = strings.TrimSpace(r.PostFormValue("name"))
	s.Host = strings.TrimSpace(r.PostFormValue("host"))
	s.SSHUser = strings.TrimSpace(orDefault(r.PostFormValue("ssh_user"), "root"))
	s.Project = strings.TrimSpace(orDefault(r.PostFormValue("project"), "frps"))
	s.RemoteBase = strings.TrimSpace(orDefault(r.PostFormValue("remote_base"), "/root/inut-jsc"))
	s.Domain = strings.TrimSpace(r.PostFormValue("domain"))
	s.CertEmail = strings.TrimSpace(r.PostFormValue("cert_email"))
	s.DashUser = strings.TrimSpace(orDefault(r.PostFormValue("dash_user"), "admin"))
	if v := r.PostFormValue("dash_password"); v != "" {
		s.DashPassword = v
	}
	if ak := r.PostFormValue("auth_kind"); ak != "" {
		s.AuthKind = ak
	}
	s.KeyRef = strings.TrimSpace(r.PostFormValue("key_ref"))

	s.SSHPort = parseIntDefault(r.PostFormValue("ssh_port"), s.SSHPort)
	s.BindPort = parseIntDefault(r.PostFormValue("bind_port"), s.BindPort)
	s.DashboardPort = parseIntDefault(r.PostFormValue("dashboard_port"), s.DashboardPort)
	s.VhostHTTPPort = parseIntDefault(r.PostFormValue("vhost_http_port"), s.VhostHTTPPort)
	s.FpmuPort = parseIntDefault(r.PostFormValue("fpmu_port"), s.FpmuPort)
	s.BindUDPPort = parseIntDefault(r.PostFormValue("bind_udp_port"), s.BindUDPPort)
	s.VhostHTTPSPort = parseIntDefault(r.PostFormValue("vhost_https_port"), s.VhostHTTPSPort)
	s.TCPMuxHTTPConnectPort = parseIntDefault(r.PostFormValue("tcpmux_httpconnect_port"), s.TCPMuxHTTPConnectPort)
	s.TCPMux = r.PostFormValue("tcp_mux") != ""

	if s.Name == "" || s.Host == "" {
		return s, errors.New("Name and Host are required")
	}
	return s, nil
}

func parseIntDefault(s string, def int) int {
	if s == "" {
		return def
	}
	if v, err := strconv.Atoi(s); err == nil {
		return v
	}
	return def
}

func orDefault(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

func friendlyDBErr(err error) string {
	if strings.Contains(err.Error(), "UNIQUE") {
		return "A server with this host and project already exists."
	}
	return err.Error()
}
