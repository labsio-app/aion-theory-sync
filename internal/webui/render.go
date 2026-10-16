package webui

import (
	"embed"
	"html/template"
	"io"
	"io/fs"
	"net/http"
)

//go:embed templates/screens.html
var templatesFS embed.FS

//go:embed templates/base.css templates/app.js templates/img
var assetsFS embed.FS

var tmpl = template.Must(template.ParseFS(templatesFS, "templates/screens.html"))

// Render writes the full HTML page for p to w.
func Render(w io.Writer, p Page) error {
	return tmpl.ExecuteTemplate(w, "shell", p)
}

// AssetsHandler serves the static CSS/JS every page links to, at
// /assets/<name> — the same handler mounted by both the real app's local
// server (main_windows.go) and the preview tool (cmd/gui-preview).
func AssetsHandler() http.Handler {
	sub, err := fs.Sub(assetsFS, "templates")
	if err != nil {
		panic(err) // only fails if the embed directive above is wrong
	}
	return http.FileServer(http.FS(sub))
}
