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
	// Use IMMEDIATE transactions because they are better
	// https://kerkour.com/sqlite-for-servers
	db, err = sql.Open("sqlite3", path+"?_txlock=immediate")
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
	// Table containing all requested URLs, to store URLs that are known but have
	// no content, like redirects.
	_, err = db.Exec(
		`CREATE TABLE IF NOT EXISTS url_log (
			url TEXT PRIMARY KEY,
			crawled_at DATETIME
		)`,
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

// InsertPage adds the given page to the database.
// It assumes you have already logged the page URL.
// If a page with that URL already exists in the database, it will be replaced.
func InsertPage(page *Page) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var pageExists bool
	err = tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM pages WHERE url = ?)`, page.URL).Scan(&pageExists)
	if err != nil {
		return err
	}

	if pageExists {
		_, err = tx.Exec(
			`UPDATE pages SET title = ?, body = ?, crawled_at = ?
			WHERE url = ?`,
			page.Title, page.Body, page.CrawledAt.Format(time.RFC3339),
			page.URL,
		)
	} else {
		_, err = tx.Exec(
			`INSERT INTO pages VALUES (?,?,?,?)`,
			page.URL, page.Title, page.Body, page.CrawledAt.Format(time.RFC3339),
		)
	}
	if err != nil {
		return err
	}

	return tx.Commit()
}

func LogURL(url string, crawledAt time.Time) error {
	_, err := db.Exec(
		`INSERT INTO url_log VALUES (?,?)
		ON CONFLICT(url) DO UPDATE SET crawled_at=?`,
		url, crawledAt, crawledAt,
	)
	return err
}

// WhenCrawled returns when the given URL was last added to the crawl log.
// The zero time.Time value is returned if it was never added.
func WhenCrawled(url string) (time.Time, error) {
	var crawledAt time.Time
	err := db.QueryRow(`SELECT crawled_at FROM url_log WHERE url = ?`, url).Scan(&crawledAt)
	if err == sql.ErrNoRows {
		return time.Time{}, nil
	}
	return crawledAt, err
}

// QueryPages returns results for the given FTS5 query string.
// The Body field of Page is never included for efficiency.
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
