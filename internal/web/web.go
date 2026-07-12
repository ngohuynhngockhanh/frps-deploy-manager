// Package web embeds the UI: HTML templates and static assets (CSS, htmx).
package web

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"path"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed static/*
var staticFS embed.FS

// Renderer holds one parsed template set per page, each composed with the
// shared base layout so page-level {{define "title"}} / {{define "content"}}
// overrides don't collide across pages.
type Renderer struct {
	pages map[string]*template.Template
}

// NewRenderer parses base.html together with every other page template.
func NewRenderer() (*Renderer, error) {
	entries, err := fs.Glob(templatesFS, "templates/*.html")
	if err != nil {
		return nil, err
	}
	pages := make(map[string]*template.Template)
	for _, f := range entries {
		name := path.Base(f)
		if name == "base.html" {
			continue
		}
		t, err := template.New(name).Funcs(funcs()).ParseFS(templatesFS, "templates/base.html", f)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", name, err)
		}
		pages[name] = t
	}
	return &Renderer{pages: pages}, nil
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
