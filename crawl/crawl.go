package crawl

import (
	"bytes"
	"log/slog"
	"mime"
	"net/http"
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
	slog.Debug("crawl.Crawl", "len(urls)", len(urls))

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
	c.OnError(func(r *colly.Response, err error) {
		slog.Warn("http error", "url", r.Request.URL, "err", err)
	})
	c.OnRequest(func(r *colly.Request) {
		slog.Info("visited", "id", r.ID, "url", r.URL)
		// Log initial URL
		if err := database.LogURL(r.URL.String(), time.Now()); err != nil {
			slog.Error("database.LogURL", "url", r.URL, "err", err)
		}
	})
	c.SetRedirectHandler(func(req *http.Request, via []*http.Request) error {
		// Just log redirect URLs to reduce re-crawling of them in the future
		if err := database.LogURL(req.URL.String(), time.Now()); err != nil {
			slog.Error("database.LogURL", "url", req.URL, "err", err)
		}
		// Keep max of 10 redirects
		// https://github.com/gocolly/colly/blob/bbf3f10c37205136e9d4f46fe8118205cc505a67/colly.go#L1270
		if len(via) >= 10 {
			return http.ErrUseLastResponse
		}
		return nil
	})

	q, _ := queue.New(5, nil) // This is from lieu too
	for _, url := range urls {
		q.AddURL(url)
	}
	return q.Run(c)
}

func onResponse(r *colly.Response) {
	// Skip errors, extract title, convert to plain text, insert in database

	slog.Debug("got response", "id", r.Request.ID, "url", r.Request.URL)

	if r.StatusCode != 200 {
		// XXX: seemingly this never runs, I guess redirects and errors are already handled
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

	crawledAt := time.Now()
	err := database.LogURL(r.Request.URL.String(), crawledAt)
	if err != nil {
		slog.Error("database.LogURL", "url", r.Request.URL, "err", err)
		// Continuing after this error is tolerable
	}
	err = database.InsertPage(&database.Page{
		URL:       r.Request.URL.String(),
		Title:     title,
		Body:      plain,
		CrawledAt: crawledAt,
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
