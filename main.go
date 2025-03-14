package main

import (
	"bufio"
	"embed"
	"log/slog"
	"math/rand/v2"
	urlPkg "net/url"
	"os"
	"strings"
	"time"

	"github.com/makew0rld/search/crawl"
	"github.com/makew0rld/search/database"
	"github.com/makew0rld/search/server"
)

//go:embed templates
var assetFS embed.FS

// URLs crawled younger than this will not be recrawled
const recrawlInterval = time.Hour * 24 * 7

func main() {
	slog.SetLogLoggerLevel(slog.LevelDebug)

	if len(os.Args) == 1 {
		slog.Error("provide subcommand")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "index":
		if err := database.Init("index.db"); err != nil {
			slog.Error("database.Init", "err", err)
			os.Exit(1)
		}

		f, err := os.Open(os.Args[2])
		if err != nil {
			slog.Error("file open", "err", err)
			os.Exit(1)
		}
		defer f.Close()

		urls := make([]string, 0)
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			// Skip blank lines
			if len(strings.TrimSpace(line)) == 0 {
				continue
			}
			// Skip comments
			if line[0] == '#' {
				continue
			}

			u, err := urlPkg.Parse(line)
			if err != nil {
				slog.Error("URL parse", "line", line, "err", err)
				os.Exit(1)
			}
			// Normalize URL: require path, remove fragment
			if u.Path == "" {
				u.Path = "/"
			}
			u.Fragment = ""
			line = u.String()

			// Don't re-crawl recent URLs
			crawledAt, err := database.WhenCrawled(line)
			if err != nil {
				slog.Error("database.WhenCrawled", "err", err)
				os.Exit(1)
			}
			if crawledAt.After(time.Now().Add(-recrawlInterval)) {
				slog.Debug("ingest: skipping already crawled recently", "url", line)
				continue
			}

			urls = append(urls, line)
		}
		f.Close()
		if err := scanner.Err(); err != nil {
			slog.Error("file read", "err", err)
			os.Exit(1)
		}

		// Shuffle URLs to prevent repeated requests to the same domain if it's sorted
		rand.Shuffle(len(urls), func(i, j int) {
			urls[i], urls[j] = urls[j], urls[i]
		})

		if err := crawl.Crawl(urls); err != nil {
			slog.Error("crawl.Crawl", "err", err)
			os.Exit(1)
		}
	case "serve":
		if err := database.Init("index.db"); err != nil {
			slog.Error("database.Init", "err", err)
			os.Exit(1)
		}
		if err := server.Serve(":8000", assetFS); err != nil {
			slog.Error("server.Serve", "err", err)
			os.Exit(1)
		}
	default:
		slog.Error("unknown command")
		os.Exit(1)
	}
}
