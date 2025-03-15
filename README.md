# Personal search engine

This is a personal search engine I developed in Go.

Given a list of URLs, it crawls only those pages and stores them in a SQLite database.
Then you can run the web server to search the pages.

![screenshot of search results](./screenshot.png)

This project was inspired by [technomancy search](https://search.technomancy.us/).

## Usage

```bash
# First index pages
./search index urls.txt

# Then serve it for searching
./search serve
```

## Install

This is solely a personal project, and currently has some rough edges,
for example there is no config file.

You can install it by cloning the git repo and compiling it yourself with
the command `go build -tags fts5`.

The binaries [`pandoc`](https://pandoc.org/) and `pdftotext` are required. The latter can be
installed from the `poppler` or `poppler-utils` package on your Linux distribution.

## License

This project is licensed under the MIT license.
