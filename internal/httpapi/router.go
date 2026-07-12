// Package httpapi wires HTTP routes (pages + JSON API) to the backend.
package httpapi

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/ngohuynhngockhanh/frps-deploy-manager/internal/web"
)

// Deps are the collaborators the HTTP layer needs. It grows across milestones.
type Deps struct {
	Render *web.Renderer
}

// NewRouter builds the HTTP handler tree.
func NewRouter(d Deps) http.Handler {
	mux := http.NewServeMux()

	mux.Handle("GET /static/", web.StaticHandler())
	mux.HandleFunc("GET /healthz", healthz)
	mux.HandleFunc("GET /{$}", d.index)

	return logging(mux)
}

func healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (d Deps) index(w http.ResponseWriter, _ *http.Request) {
	if err := d.Render.Render(w, "index.html", nil); err != nil {
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
