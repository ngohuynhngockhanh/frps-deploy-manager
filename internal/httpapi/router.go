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
	protected.HandleFunc("GET /servers/new", d.serverNew)
	protected.HandleFunc("POST /servers", d.serverCreate)
	protected.HandleFunc("GET /servers/{id}", d.serverDetail)
	protected.HandleFunc("GET /servers/{id}/edit", d.serverEdit)
	protected.HandleFunc("POST /servers/{id}", d.serverUpdate)
	protected.HandleFunc("POST /servers/{id}/delete", d.serverDelete)
	protected.HandleFunc("POST /servers/{id}/test-ssh", d.testSSH)
	protected.HandleFunc("POST /servers/{id}/deploy", d.deploy)
	protected.HandleFunc("GET /servers/{id}/frp-users", d.frpUsersList)
	protected.HandleFunc("POST /servers/{id}/frp-users", d.frpUserAdd)
	protected.HandleFunc("POST /servers/{id}/frp-users/{uid}/rotate", d.frpUserRotate)
	protected.HandleFunc("POST /servers/{id}/frp-users/{uid}/delete", d.frpUserDelete)
	protected.HandleFunc("GET /servers/{id}/certs", d.certPanel)
	protected.HandleFunc("POST /servers/{id}/certs/issue", d.certIssue)
	protected.HandleFunc("POST /servers/{id}/certs/refresh", d.certRefresh)
	mux.Handle("/", d.Auth.Middleware(protected))

	return logging(mux)
}

func healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (d Deps) index(w http.ResponseWriter, r *http.Request) {
	servers, err := d.Store.ListServers()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := d.Render.Render(w, "index.html", d.page(r, servers)); err != nil {
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
