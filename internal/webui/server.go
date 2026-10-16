package webui

import (
	"fmt"
	"html"
	"net/http"
	"strings"
)

// Server is the local HTTP server the real app's WebView2 window
// (main_windows.go) navigates to: Provide returns whatever Page the
// current Machine/Extra should show right now, re-read on every request —
// so a client-side reload after an action always sees the latest state,
// the same role cmd/gui's old a.render() played by rebuilding a walk
// widget tree.
type Server struct {
	Provide func() Page
	mux     *http.ServeMux
}

func NewServer(provide func() Page) *Server {
	s := &Server{Provide: provide, mux: http.NewServeMux()}
	s.mux.Handle("/assets/", http.StripPrefix("/assets/", AssetsHandler()))
	s.mux.HandleFunc("/", s.handleIndex)
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := Render(w, s.Provide()); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// NewPreviewHandler serves every fixture at /preview/<name>, plus an index
// at / listing them — used by cmd/gui-preview (a manual-review tool) and
// by tools/visual's screenshot harness, neither of which is the real app:
// no /action/* here, since nothing on these pages is wired to a Machine.
func NewPreviewHandler(fixtures []Fixture) http.Handler {
	byName := make(map[string]Fixture, len(fixtures))
	for _, f := range fixtures {
		byName[f.Name] = f
	}

	mux := http.NewServeMux()
	mux.Handle("/assets/", http.StripPrefix("/assets/", AssetsHandler()))
	mux.HandleFunc("/preview/", func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/preview/")
		f, ok := byName[name]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := Render(w, f.Page()); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, "<!doctype html><html><head><meta charset=utf-8>"+
			"<title>Aion Theory Sync — aperçu</title>"+
			"<style>body{background:#08071a;color:#f2eefa;font-family:system-ui;padding:24px}"+
			"a{display:block;color:#6bd5ff;padding:6px 0;font-size:14px}</style></head><body>"+
			"<h1>Écrans — aperçu</h1>")
		for _, f := range fixtures {
			fmt.Fprintf(w, "<a href=\"/preview/%s\">%s</a>", html.EscapeString(f.Name), html.EscapeString(f.Name))
		}
		fmt.Fprint(w, "</body></html>")
	})
	return mux
}
