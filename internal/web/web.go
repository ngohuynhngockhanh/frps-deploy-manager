// Package web embeds the UI: HTML templates and static assets (CSS, htmx).
package web

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed static/*
var staticFS embed.FS

// Renderer holds one parsed template set per page (each composed with the
// shared base layout) plus standalone partials (files named _*.html) rendered
// bare for htmx swaps.
type Renderer struct {
	pages    map[string]*template.Template
	partials map[string]*template.Template
}

// NewRenderer parses base.html with every page template, and each _*.html
// partial on its own.
func NewRenderer() (*Renderer, error) {
	entries, err := fs.Glob(templatesFS, "templates/*.html")
	if err != nil {
		return nil, err
	}
	r := &Renderer{
		pages:    make(map[string]*template.Template),
		partials: make(map[string]*template.Template),
	}
	for _, f := range entries {
		name := path.Base(f)
		switch {
		case name == "base.html":
			continue
		case strings.HasPrefix(name, "_"):
			t, err := template.New(name).Funcs(funcs()).ParseFS(templatesFS, f)
			if err != nil {
				return nil, fmt.Errorf("parse partial %s: %w", name, err)
			}
			r.partials[name] = t
		default:
			t, err := template.New(name).Funcs(funcs()).ParseFS(templatesFS, "templates/base.html", f)
			if err != nil {
				return nil, fmt.Errorf("parse %s: %w", name, err)
			}
			r.pages[name] = t
		}
	}
	return r, nil
}

// Render writes the named page (e.g. "index.html") using the base layout.
func (r *Renderer) Render(w http.ResponseWriter, page string, data any) error {
	t, ok := r.pages[page]
	if !ok {
		return fmt.Errorf("unknown page %q", page)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return t.ExecuteTemplate(w, "base", data)
}

// RenderPartial writes a bare partial (e.g. "_testssh.html") for htmx swaps.
func (r *Renderer) RenderPartial(w http.ResponseWriter, name string, data any) error {
	t, ok := r.partials[name]
	if !ok {
		return fmt.Errorf("unknown partial %q", name)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return t.ExecuteTemplate(w, name, data)
}

// StaticHandler serves embedded static assets; mount under /static/.
func StaticHandler() http.Handler {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err) // embedded FS is fixed at build time
	}
	return http.StripPrefix("/static/", http.FileServer(http.FS(sub)))
}

func funcs() template.FuncMap {
	return template.FuncMap{}
}
