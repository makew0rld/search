package server

import (
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/makew0rld/search/database"
)

var tmpls *template.Template

func rootHandler(w http.ResponseWriter, r *http.Request) {
	err := tmpls.ExecuteTemplate(w, "index.tmpl", nil)
	if err != nil {
		slog.Error("rootHandler", "err", err)
		http.Error(w, err.Error(), 500)
		return
	}
}

type searchResult struct {
	Title     string
	URL       string
	CrawledAt time.Time
	Host      string
}

func searchResultFromPage(p *database.Page) *searchResult {
	var host string
	u, err := url.Parse(p.URL)
	if err == nil {
		host = u.Host
	}
	return &searchResult{
		Title:     p.Title,
		URL:       p.URL,
		CrawledAt: p.CrawledAt,
		Host:      host,
	}
}

// sanitizeQuery prepares user input for the database
// It assumes you just want to search for a group of unordered words
func sanitizeQuery(query string) string {
	// https://www.sqlite.org/fts5.html#full_text_query_syntax
	// https://stackoverflow.com/a/79510332/7361270
	query = strings.ReplaceAll(query, `"`, `""`) // Escape quotes
	words := strings.Fields(query)
	// Quote each word
	return `"` + strings.Join(words, `" "`) + `"`
}

func searchHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		slog.Debug("searchHandler", "err", err)
		http.Error(w, err.Error(), 400)
		return
	}
	query := r.Form.Get("q")
	if query == "" {
		slog.Debug("searchHandler: no query provided")
		http.Error(w, "no query provided", 400)
		return
	}
	slog.Debug("searchHandler", "query", query)
	sanitizedQuery := sanitizeQuery(query)
	slog.Debug("searchHandler", "sanitized", sanitizedQuery)
	pages, err := database.QueryPages(sanitizedQuery)
	if err != nil {
		slog.Error("database.QueryPages", "query", query, "err", err)
		http.Error(w, err.Error(), 500)
		return
	}

	data := struct {
		Results []*searchResult
		Query   string
	}{
		Results: make([]*searchResult, len(pages)),
		Query:   query,
	}
	for i, page := range pages {
		data.Results[i] = searchResultFromPage(page)
	}

	err = tmpls.ExecuteTemplate(w, "search.tmpl", data)
	if err != nil {
		slog.Error("searchHandler", "err", err)
		http.Error(w, err.Error(), 500)
		return
	}
}

func Serve(addr string, assetFS fs.FS) error {
	tmpls = template.Must(template.ParseFS(assetFS, "templates/*.tmpl"))

	http.HandleFunc("GET /", rootHandler)
	http.HandleFunc("GET /search", searchHandler)

	slog.Info("starting server", "addr", addr)
	return http.ListenAndServe(addr, nil)
}
