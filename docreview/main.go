// Command docreview serves the repo's Markdown docs as HTML for review.
package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/alecthomas/kong"
)

type cli struct {
	Repo string `help:"Git repository to read docs from." default:"." env:"DOCREVIEW_REPO" type:"path"`
	Addr string `help:"Listen address." default:"127.0.0.1:8080" env:"DOCREVIEW_ADDR"`
}

func main() {
	var c cli
	kong.Parse(&c, kong.Name("docreview"))
	os.Exit(runServer(c, os.Stderr))
}

// runServer installs logging on f, serves, and returns the process exit code.
func runServer(c cli, f *os.File) int {
	setupLogging(f)
	if err := serve(c); err != nil {
		slog.Error("server stopped", "err", err)
		return 1
	}
	return 0
}

func serve(c cli) error {
	slog.Info("listening", "addr", "http://"+c.Addr, "repo", c.Repo)
	// Local viewer: requests are tiny; WriteTimeout leaves room for a git read plus render.
	srv := &http.Server{
		Addr:              c.Addr,
		Handler:           NewServer(c.Repo).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	return srv.ListenAndServe()
}
