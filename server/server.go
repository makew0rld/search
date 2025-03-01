package server

import (
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
)

var tmpls *template.Template

func rootHandler(w http.ResponseWriter, r *http.Request) {
	err := tmpls.ExecuteTemplate(w, "index.tmpl", nil)
	if err != nil {
		slog.Error("rootHandler", "err", err)
		http.Error(w, err.Error(), 500)
	}
}

func Serve(addr string, assetFS fs.FS) error {
	tmpls = template.Must(template.ParseFS(assetFS, "templates/*.tmpl"))

	http.HandleFunc("GET /", rootHandler)

	slog.Info("listening on", "addr", addr)
	return http.ListenAndServe(addr, nil)
}
