package main

import (
	"bufio"
	"log/slog"
	"math/rand/v2"
	"os"
	"strings"

	"github.com/makew0rld/search/crawl"
	"github.com/makew0rld/search/database"
)

func main() {
	slog.SetLogLoggerLevel(slog.LevelDebug)

	f, err := os.Open(os.Args[1])
	if err != nil {
		slog.Error("file open", "err", err)
		os.Exit(1)
	}
	defer f.Close()

	urls := make([]string, 0)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		// Skip comments
		if line[0] == '#' {
			continue
		}
		// Skip blank lines
		if len(strings.TrimSpace(line)) == 0 {
			continue
		}
		urls = append(urls, line)
	}
	f.Close()
	if err := scanner.Err(); err != nil {
		slog.Error("file read", "err", err)
		os.Exit(1)
	}

	if err := database.Init("index.db"); err != nil {
		slog.Error("database.Init", "err", err)
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
}
