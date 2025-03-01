package database

import (
	"database/sql"
	"log/slog"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

func Init(path string) error {
	var err error
	db, err = sql.Open("sqlite3", path)
	if err != nil {
		return err
	}
	createTables()
	return nil
}

func createTables() {
	_, err := db.Exec(
		`CREATE VIRTUAL TABLE IF NOT EXISTS pages
		USING fts5(url, title, body, crawled_at UNINDEXED, tokenize = porter)`,
	)
	if err != nil {
		slog.Error("createTables", "err", err)
		os.Exit(1)
	}
}

type Page struct {
	URL       string
	Title     string
	Body      string
	CrawledAt time.Time
}

func InsertPage(page *Page) error {
	_, err := db.Exec(
		`INSERT INTO pages VALUES (?,?,?,?)`,
		page.URL, page.Title, page.Body, page.CrawledAt.Format(time.RFC3339),
	)
	return err
}

// QueryPages returns results for the given FTS5 query string.
//
//	The Body field of Page is never included for efficiency.
func QueryPages(query string) ([]*Page, error) {
	rows, err := db.Query(
		`SELECT url, title, crawled_at FROM pages WHERE pages MATCH ? ORDER BY rank`,
		query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	pages := make([]*Page, 0)
	var crawledAt string
	for rows.Next() {
		var page Page
		err := rows.Scan(&page.URL, &page.Title, &crawledAt)
		if err != nil {
			return nil, err
		}
		page.CrawledAt, _ = time.Parse(time.RFC3339, crawledAt)
		pages = append(pages, &page)
	}
	return pages, rows.Err()
}
