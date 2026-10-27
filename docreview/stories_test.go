package main

import (
	"bytes"
	"html"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const epicsFixture = "# Epics\n\n| Story | AC |\n|---|---|\n| **E1S1** Structs | needs E1S2 |\n| **E1S2** Decoder | see E9S9 |\n"

func TestStoryLinks(t *testing.T) {
	repo, _ := fixture(t)
	write(t, repo, "docs/epics.md", epicsFixture)
	write(t, repo, "docs/s.md", "Plain E1S1, again E1S2.\n\n| c |\n|---|\n| E1S1 in cell |\n\n`E1S1` [E1S2](x.md)\n\n```\nE1S1\n```\n\nunknown E9S9 and XE1S1 and E1S1X.\n\n## E1S1 heading\n")
	write(t, repo, "AGENTS.md", "see E1S2\n")
	run(t, repo, "add", ".")
	run(t, repo, "commit", "-m", "stories")
	srv := srvFor(repo)

	_, body := get(t, repo, "/?s=b:main&p=docs/s.md")
	link := regexp.MustCompile(`<a href="(/\?[^"]*)#(e1s\d)">(E1S\d)</a>`)
	ms := link.FindAllStringSubmatch(body, -1)
	if len(ms) != 4 { // plain x2, cell, heading
		t.Fatalf("want 4 story links, got %d in %s", len(ms), body)
	}
	for _, m := range ms {
		if strings.ToLower(m[3]) != m[2] {
			t.Errorf("text %s links to %s", m[3], m[2])
		}
		u, _ := url.Parse(html.UnescapeString(m[1]))
		code, page := get(t, repo, u.String())
		if code != 200 || !strings.Contains(page, `id="`+m[2]+`"`) {
			t.Errorf("link %s: %d, anchor missing: %.200s", u.String(), code, page)
		}
		q := u.Query()
		if q.Get("p") != "docs/epics.md" || q.Get("ps") != srv.sigDoc("docs/epics.md") || q.Get("ss") != srv.sigSource("b:main") {
			t.Errorf("bad link %s", m[1])
		}
	}
	for _, no := range []string{"<code>E1S1</code>", `>E1S2</a>`, "E1S1\n</code>", "E9S9", "XE1S1", "E1S1X"} {
		if !strings.Contains(body, no) {
			t.Errorf("expected untouched %q", no)
		}
	}
	if strings.Contains(body, ">E9S9</a>") || strings.Contains(body, ">E1S1X</a>") || strings.Contains(body, `<a href="#e1s1"`) {
		t.Error("linked what must stay plain")
	}

	_, body = get(t, repo, "/?s=b:main&p=docs/epics.md")
	for _, want := range []string{`<td id="e1s1">`, `<td id="e1s2">`, `<a href="#e1s2">E1S2</a>`} {
		if !strings.Contains(body, want) {
			t.Errorf("epics.md: missing %s in %s", want, body)
		}
	}
	if strings.Contains(body, `href="#e1s1"`) || strings.Contains(body, `<a href="/?`) && strings.Contains(body, ">E1S1</a>") {
		t.Error("anchored bold ID must not link to itself")
	}
	_, body = get(t, repo, "/?s=b:main&p=AGENTS.md")
	if !strings.Contains(body, `#e1s2">E1S2</a>`) {
		t.Error("AGENTS.md not linked")
	}
}

func TestNoEpicsNoLinks(t *testing.T) {
	repo, _ := fixture(t)
	write(t, repo, "docs/s.md", "E1S1\n")
	run(t, repo, "add", ".")
	run(t, repo, "commit", "-m", "s")
	_, body := get(t, repo, "/?s=b:main&p=docs/s.md")
	if strings.Contains(body, "<a ") {
		t.Error("linked without epics.md")
	}
}

func TestUnreadableEpicsWarnsAndRenders(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores permissions")
	}
	repo, wt := fixture(t)
	write(t, wt, "docs/epics.md", epicsFixture)
	write(t, wt, "docs/s.md", "E1S1\n")
	if err := os.Chmod(filepath.Join(wt, "docs", "epics.md"), 0); err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	old := slog.Default()
	defer slog.SetDefault(old)
	slog.SetDefault(slog.New(slog.NewTextHandler(&b, nil)))
	code, body := get(t, repo, "/?s=w:"+wt+"&p=docs/s.md")
	if code != 200 || strings.Contains(body, ">E1S1</a>") || !strings.Contains(b.String(), "epics.md unreadable") {
		t.Errorf("code %d, log %q", code, b.String())
	}
}

func TestStoryLinksKeepDecoding(t *testing.T) {
	repo, _ := fixture(t)
	write(t, repo, "docs/epics.md", epicsFixture)
	write(t, repo, "docs/s.md", "a &copy; \\* E1S1 \\* &copy; b\n")
	run(t, repo, "add", ".")
	run(t, repo, "commit", "-m", "s")
	_, body := get(t, repo, "/?s=b:main&p=docs/s.md")
	if !regexp.MustCompile(`a © \* <a href="[^"]*#e1s1">E1S1</a> \* © b`).MatchString(body) {
		t.Errorf("entities or escapes around a story link not decoded: %s", body)
	}
}
