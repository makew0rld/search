package crawl

import (
	"bytes"
	"log/slog"
	"mime"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
	"github.com/gocolly/colly/v2/queue"
	"github.com/makew0rld/search/database"
)

// Crawl gets web page content and inserts it into the database.
// It is expected the given URL list will have already been sanitized,
// including to remove URLs that were previously crawled in the database and so
// should be skipped.
func Crawl(urls []string) error {
	c := colly.NewCollector(
		colly.MaxDepth(1),
		colly.UserAgent("makeworld personal search"),
	)
	c.IgnoreRobotsTxt = false

	// Unfortunately this is not a per domain/IP limit, but across all domains, I think
	// These values are pulled from cblgh/lieu and should be investigated
	c.Limit(&colly.LimitRule{DomainGlob: "*", Parallelism: 3, Delay: 200 * time.Millisecond})

	// The heart of the crawl logic
	c.OnResponse(onResponse)

	q, _ := queue.New(5, nil) // This too
	for _, url := range urls {
		q.AddURL(url)
	}
	return q.Run(c)
}

func onResponse(r *colly.Response) {
	// Skip errors, extract title, convert to plain text, insert in database

	slog.Info("visiting", "id", r.Request.ID, "url", r.Request.URL)

	if r.StatusCode != 200 {
		slog.Warn("http", "code", r.StatusCode, "url", r.Request.URL)
		return
	}

	plain := ""
	title := ""

	mediaType, _, _ := mime.ParseMediaType(r.Headers.Get("Content-Type"))
	switch mediaType {
	case "text/html":
		var err error
		plain, err = htmlToPlain(r.Body)
		if err != nil {
			slog.Error("pandoc", "url", r.Request.URL, "err", err)
			return
		}

		doc, err := goquery.NewDocumentFromReader(bytes.NewReader(r.Body))
		if err != nil {
			slog.Error("goquery", "url", r.Request.URL, "err", err)
			return
		}
		title = doc.Find("title").First().Text()
		if title == "" {
			title = doc.Find("h1").First().Text()
		}
		title = strings.TrimSpace(title)
	case "application/pdf":
		var err error
		plain, err = pdfToPlain(r.Body)
		if err != nil {
			slog.Error("pdftotext", "url", r.Request.URL, "err", err)
			return
		}
		title = path.Base(r.Request.URL.Path)
	case "text/plain":
		plain = string(r.Body)
		title = path.Base(r.Request.URL.Path)
	default:
		slog.Warn("unknown mediatype", "Content-Type", r.Headers.Get("Content-Type"), "url", r.Request.URL)
		return
	}
	slog.Debug("set title", "title", title, "url", r.Request.URL)

	err := database.InsertPage(&database.Page{
		URL:       r.Request.URL.String(),
		Title:     title,
		Body:      plain,
		CrawledAt: time.Now(),
	})
	if err != nil {
		slog.Error("database.InsertPage", "url", r.Request.URL, "err", err)
	}
}

func htmlToPlain(body []byte) (string, error) {
	cmd := exec.Command("pandoc", "--quiet", "--sandbox", "-f", "html", "-t", "plain")
	cmd.Stdin = bytes.NewReader(body)
	var out strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	return out.String(), err
}

func pdfToPlain(body []byte) (string, error) {
	cmd := exec.Command("pdftotext", "-", "-")
	cmd.Stdin = bytes.NewReader(body)
	var out strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	return out.String(), err
}
