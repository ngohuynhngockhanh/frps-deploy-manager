// Package httpapi wires HTTP routes (pages + JSON API) to the backend.
package httpapi

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/ngohuynhngockhanh/frps-deploy-manager/internal/auth"
	"github.com/ngohuynhngockhanh/frps-deploy-manager/internal/crypto"
	"github.com/ngohuynhngockhanh/frps-deploy-manager/internal/remote"
	"github.com/ngohuynhngockhanh/frps-deploy-manager/internal/store"
	"github.com/ngohuynhngockhanh/frps-deploy-manager/internal/web"
)

// Deps are the collaborators the HTTP layer needs. It grows across milestones.
type Deps struct {
	Render   *web.Renderer
	Store    *store.Store
	Auth     *auth.Manager
	Deployer *remote.Deployer
	Sealer   *crypto.Sealer
}

// NewRouter builds the HTTP handler tree: public routes (login, static,
// health) plus a session-protected subtree for everything else.
func NewRouter(d Deps) http.Handler {
	mux := http.NewServeMux()

	// Public.
	mux.Handle("GET /static/", web.StaticHandler())
	mux.HandleFunc("GET /healthz", healthz)
	mux.HandleFunc("GET /login", d.loginPage)
	mux.HandleFunc("POST /login", d.loginSubmit)

	// Protected subtree (session + CSRF enforced by Middleware).
	protected := http.NewServeMux()
	protected.HandleFunc("GET /{$}", d.index)
	protected.HandleFunc("POST /logout", d.logout)
	// Admin-only: server lifecycle, deploy, certs, DNS.
	protected.HandleFunc("GET /servers/new", requireAdmin(d.serverNew))
	protected.HandleFunc("POST /servers", requireAdmin(d.serverCreate))
	protected.HandleFunc("GET /servers/{id}/edit", requireAdmin(d.serverEdit))
	protected.HandleFunc("POST /servers/{id}", requireAdmin(d.serverUpdate))
	protected.HandleFunc("POST /servers/{id}/delete", requireAdmin(d.serverDelete))
	protected.HandleFunc("POST /servers/{id}/test-ssh", requireAdmin(d.testSSH))
	protected.HandleFunc("POST /servers/{id}/deploy", requireAdmin(d.deploy))
	protected.HandleFunc("POST /servers/{id}/certs/issue", requireAdmin(d.certIssue))
	protected.HandleFunc("POST /servers/{id}/certs/refresh", requireAdmin(d.certRefresh))
	protected.HandleFunc("POST /servers/{id}/dns/cloudflare", requireAdmin(d.certPointDNS))
	protected.HandleFunc("POST /servers/{id}/certs/cloudflare", requireAdmin(d.certIssueCloudflare))
	// Manager-allowed on assigned sites: view + frp-user management.
	protected.HandleFunc("GET /servers/{id}", d.serverDetail)
	protected.HandleFunc("GET /servers/{id}/frp-users", d.frpUsersList)
	protected.HandleFunc("POST /servers/{id}/frp-users", d.frpUserAdd)
	protected.HandleFunc("POST /servers/{id}/frp-users/{uid}/rotate", d.frpUserRotate)
	protected.HandleFunc("POST /servers/{id}/frp-users/{uid}/delete", d.frpUserDelete)
	protected.HandleFunc("GET /servers/{id}/certs", d.certPanel)
	// Account management (admin-only).
	protected.HandleFunc("GET /accounts", requireAdmin(d.accountsPage))
	protected.HandleFunc("POST /accounts", requireAdmin(d.accountCreate))
	protected.HandleFunc("POST /accounts/{id}/delete", requireAdmin(d.accountDelete))
	protected.HandleFunc("POST /accounts/{id}/password", requireAdmin(d.accountPassword))
	protected.HandleFunc("POST /accounts/{id}/servers", requireAdmin(d.accountServers))
	// frps dashboard reverse proxy (Basic-auth injected). /api/* is the frp
	// dashboard SPA's API namespace, unused by this admin UI.
	protected.HandleFunc("/dash/", d.dashProxy)
	protected.HandleFunc("/api/", d.dashProxy)
	mux.Handle("/", d.Auth.Middleware(protected))

	return logging(mux)
}

// requireAdmin rejects non-admin operators.
func requireAdmin(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !auth.IsAdmin(r) {
			http.Error(w, "forbidden — admin only", http.StatusForbidden)
			return
		}
		h(w, r)
	}
}

func healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// serverCard is a dashboard row: a server plus its cert expiry summary.
type serverCard struct {
	store.Server
	HasCert  bool
	CertDays int
}

func (d Deps) index(w http.ResponseWriter, r *http.Request) {
	var servers []store.Server
	var err error
	if auth.IsAdmin(r) {
		servers, err = d.Store.ListServers()
	} else {
		servers, err = d.Store.ListServersForUser(auth.CurrentUserID(r))
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	cards := make([]serverCard, 0, len(servers))
	for _, s := range servers {
		card := serverCard{Server: s}
		if s.Domain != "" {
			if c, err := d.Store.GetCert(s.ID, s.Domain); err == nil {
				card.HasCert = true
				card.CertDays = c.DaysRemaining()
			}
		}
		cards = append(cards, card)
	}
	if err := d.Render.Render(w, "index.html", d.page(r, cards)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// logging is a tiny request logger; loopback-only tool, keep it simple.
func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		log.Printf("%s %s -> %d (%s)", r.Method, r.URL.Path, sw.status, time.Since(start).Round(time.Millisecond))
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (s *statusWriter) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// Flush lets SSE handlers stream through the wrapper.
func (s *statusWriter) Flush() {
	if f, ok := s.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
