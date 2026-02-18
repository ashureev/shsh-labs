// Package web embeds the built frontend (dist/) and provides an HTTP handler
// that serves it as a single-page application (SPA).
//
// In development, the dist/ directory won't exist — the handler will return 404
// for non-API routes, and you should use the Vite dev server instead.
package web

import (
	"embed"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
)

//go:embed all:dist
var distFS embed.FS

// SPAHandler returns an http.Handler that serves the embedded frontend.
// It serves static files from dist/, and falls back to index.html for
// any path that doesn't match a file (SPA client-side routing).
func SPAHandler() http.Handler {
	subFS, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic("web: failed to create sub filesystem: " + err.Error())
	}

	fileServer := http.FileServer(http.FS(subFS))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try to serve the file directly.
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}

		// Check if file exists in the embedded FS.
		if f, err := subFS.Open(path); err == nil {
			if closeErr := f.Close(); closeErr != nil {
				slog.Debug("web: failed to close embedded file", "path", path, "error", closeErr)
			}
			fileServer.ServeHTTP(w, r)
			return
		}

		// Not found — serve index.html for SPA routing.
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}
