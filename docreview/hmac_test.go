package main

import (
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
)

// assertSignedLink finds a rewritten link to (key, p) in body and verifies its signatures with the repo's server.
func assertSignedLink(t *testing.T, repo, body, key, p string) {
	t.Helper()
	for _, m := range regexp.MustCompile(`href="(/\?[^"]*)"`).FindAllStringSubmatch(body, -1) {
		u, err := url.Parse(html.UnescapeString(m[1]))
		if err != nil {
			continue
		}
		q := u.Query()
		if q.Get("s") == key && q.Get("p") == p {
			srv := srvFor(repo)
			if q.Get("ss") != srv.sigSource(key) || q.Get("ps") != srv.sigDoc(p) {
				t.Errorf("bad signatures in %s", m[1])
			}
			return
		}
	}
	t.Errorf("no signed link to %s %s", key, p)
}

func reqCode(srv Server, raw string) int {
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest("GET", raw, nil))
	return rec.Code
}

func TestSignedRequestsAccepted(t *testing.T) {
	repo, wt := fixture(t)
	srv := srvFor(repo)
	for _, k := range []string{"b:main", "w:" + wt} {
		for _, path := range []string{"/", "/hash"} {
			u := signURL(srv, path+"?"+url.Values{"s": {k}, "p": {"docs/a.md"}}.Encode())
			if c := reqCode(srv, u); c != 200 {
				t.Errorf("%s: %d", u, c)
			}
		}
	}
	if c := reqCode(srv, "/"); c != 200 {
		t.Errorf("plain /: %d", c)
	}
	if c := reqCode(srv, "/hash"); c != 404 {
		t.Errorf("plain /hash has no doc: %d", c)
	}
}

func TestBadSignaturesRejectedBeforeAnyWork(t *testing.T) {
	repo, wt := fixture(t)
	srv := srvFor(repo)
	other := NewServer(repo) // another instance: different secret
	flip := func(s string) string {
		if s[0] == 'A' {
			return "B" + s[1:]
		}
		return "A" + s[1:]
	}
	wkey := "w:" + wt
	mk := func(s, ss, p, ps string) string {
		return "?" + url.Values{"s": {s}, "ss": {ss}, "p": {p}, "ps": {ps}}.Encode()
	}
	good := func(s string) string { return srv.sigSource(s) }
	cases := map[string]string{
		"tampered source value":    mk("b:feature", good("b:main"), "docs/a.md", srv.sigDoc("docs/a.md")),
		"tampered path value":      mk("b:main", good("b:main"), "docs/sub/b.md", srv.sigDoc("docs/a.md")),
		"tampered source sig":      mk("b:main", flip(good("b:main")), "docs/a.md", srv.sigDoc("docs/a.md")),
		"tampered path sig":        mk("b:main", good("b:main"), "docs/a.md", flip(srv.sigDoc("docs/a.md"))),
		"missing source sig":       mk("b:main", "", "docs/a.md", srv.sigDoc("docs/a.md")),
		"missing path sig":         mk("b:main", good("b:main"), "docs/a.md", ""),
		"branch sig for worktree":  mk(wkey, srv.mac("branch", wkey), "docs/a.md", srv.sigDoc("docs/a.md")),
		"branch sig as path sig":   mk("b:main", good("b:main"), "docs/a.md", srv.mac("branch", "docs/a.md")),
		"path sig as source sig":   mk("b:main", srv.mac("doc", "b:main"), "docs/a.md", srv.sigDoc("docs/a.md")),
		"worktree sig for branch":  mk("b:main", srv.mac("worktree", "b:main"), "docs/a.md", srv.sigDoc("docs/a.md")),
		"directory sig for wt":     mk(wkey, srv.mac("directory", wkey), "docs/a.md", srv.sigDoc("docs/a.md")),
		"source sig as path sig":   mk("b:main", good("b:main"), "docs/a.md", good("b:main")),
		"other server's source":    mk("b:main", other.sigSource("b:main"), "docs/a.md", srv.sigDoc("docs/a.md")),
		"other server's path":      mk("b:main", good("b:main"), "docs/a.md", other.sigDoc("docs/a.md")),
		"unknown source kind":      mk("x:main", srv.mac("", "x:main"), "", ""),
		"unsigned worktree":        "?s=" + url.QueryEscape(wkey) + "&p=docs/a.md",
		"unsigned traversal":       "?p=../../etc/passwd",
		"unsigned source, no path": "?s=b:main",
	}
	for name, q := range cases {
		for _, path := range []string{"/", "/hash"} {
			gitCalls.Store(0)
			fsReads.Store(0)
			rec := httptest.NewRecorder()
			srv.Handler().ServeHTTP(rec, httptest.NewRequest("GET", path+q, nil))
			if rec.Code != http.StatusBadRequest {
				t.Errorf("%s %s: %d", name, path, rec.Code)
			}
			if g, f := gitCalls.Load(), fsReads.Load(); g != 0 || f != 0 {
				t.Errorf("%s %s: %d git calls, %d fs reads before verification", name, path, g, f)
			}
			if strings.Contains(rec.Body.String(), "sig") {
				t.Errorf("%s: response names the failing part: %q", name, rec.Body.String())
			}
		}
	}
}

func TestPageCarriesSignaturesForAllURLs(t *testing.T) {
	repo, _ := fixture(t)
	srv := srvFor(repo)
	_, body := get(t, repo, "/") // no params: default source and doc, freshly signed
	re := func(s string) []string {
		var out []string
		for _, m := range regexp.MustCompile(s).FindAllStringSubmatch(body, -1) {
			out = append(out, m[1:]...)
		}
		return out
	}
	srcs := re(`<option value="([bwd]:[^"]*)" data-sig="([^"]*)"`)
	docs := re(`<option value="([^"]*\.md)" data-sig="([^"]*)"`)
	if len(srcs) < 6 || len(docs) < 6 {
		t.Fatalf("options: %v %v", srcs, docs)
	}
	for i := 0; i < len(srcs); i += 2 {
		if srcs[i+1] != srv.sigSource(html.UnescapeString(srcs[i])) {
			t.Errorf("source option %q unsigned", srcs[i])
		}
	}
	for i := 0; i < len(docs); i += 2 {
		if docs[i+1] != srv.sigDoc(docs[i]) {
			t.Errorf("doc option %q unsigned", docs[i])
		}
	}
	art := re(`data-s="([^"]*)" data-ss="([^"]*)" data-p="([^"]*)" data-ps="([^"]*)"`)
	if len(art) != 4 || art[1] != srv.sigSource(html.UnescapeString(art[0])) || art[3] != srv.sigDoc(art[2]) {
		t.Errorf("article data: %v", art)
	}
	// the poll URL built from the article data is accepted
	if c := reqCode(srv, "/hash?"+url.Values{"s": {html.UnescapeString(art[0])}, "ss": {art[1]}, "p": {art[2]}, "ps": {art[3]}}.Encode()); c != 200 {
		t.Errorf("hash with page data: %d", c)
	}
	if !strings.Contains(body, "ss: art.dataset.ss") || !strings.Contains(body, "ps: art.dataset.ps") {
		t.Error("reload script does not send signatures")
	}
}
