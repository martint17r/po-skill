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

// hrefs returns the unescaped /?… links of body.
func hrefs(body string) []url.Values {
	var out []url.Values
	for _, m := range regexp.MustCompile(`href="(/\?[^"]*)"`).FindAllStringSubmatch(body, -1) {
		if u, err := url.Parse(html.UnescapeString(m[1])); err == nil {
			out = append(out, u.Query())
		}
	}
	return out
}

func hasLink(body, param, v string) bool {
	for _, q := range hrefs(body) {
		if q.Get(param) == v {
			return true
		}
	}
	return false
}

func TestDirectoryLinksRewritten(t *testing.T) {
	repo, wt := fixture(t)
	links := map[string]string{ // markdown destination (from docs/a.md) -> directory param
		"./sub/":      "docs/sub",
		"sub":         "docs/sub",
		"./sub":       "docs/sub",
		"../docs/":    "docs",
		"./":          "docs",
		"../":         ".",
		"sub/#frag":   "docs/sub",
		"../docs/sub": "docs/sub",
	}
	md := ""
	for l := range links {
		md += "[x](" + l + ") "
	}
	md += "[none](../src/) [esc](../../) [abs](/docs/)\n"
	write(t, repo, "docs/a.md", md)
	write(t, wt, "docs/a.md", md)
	run(t, repo, "commit", "-am", "dirs")
	for _, src := range []struct{ key, url string }{{"b:main", "/?s=b:main&p=docs/a.md"}, {"w:" + wt, "/?s=w:" + wt + "&p=docs/a.md"}} {
		_, body := get(t, repo, src.url)
		for l, d := range links {
			found := false
			for _, q := range hrefs(body) {
				if q.Get("d") == d && q.Get("s") == src.key && q.Get("ds") == srvFor(repo).sigDir(d) && q.Get("ss") == srvFor(repo).sigSource(src.key) {
					found = true
				}
			}
			if !found {
				t.Errorf("%s: link %q -> d=%q not rewritten", src.key, l, d)
			}
		}
		for _, want := range []string{`href="../src/"`, `href="../../"`, `href="/docs/"`} {
			if !strings.Contains(body, want) {
				t.Errorf("%s: %s must stay untouched", src.key, want)
			}
		}
	}
}

func TestDirectoryIndex(t *testing.T) {
	repo, wt := fixture(t)
	write(t, wt, "docs/sub/c.md", "# c\n")
	for _, src := range []struct{ key, base string }{{"b:main", "/?s=b:main"}, {"w:" + wt, "/?s=w:" + wt}} {
		code, body := get(t, repo, src.base+"&d=docs")
		if code != 200 {
			t.Fatalf("%s: %d", src.key, code)
		}
		if !hasLink(body, "d", "docs/sub") || !hasLink(body, "p", "docs/a.md") || hasLink(body, "p", "README.md") {
			t.Errorf("%s: wrong entries: %s", src.key, body)
		}
		// every entry link resolves
		for _, q := range hrefs(body) {
			if q.Get("d") == "" && q.Get("p") == "" {
				continue
			}
			if c, _ := get(t, repo, "/?"+q.Encode()); c != 200 {
				t.Errorf("%s: entry %v -> %d", src.key, q, c)
			}
		}
	}
	_, body := get(t, repo, "/?s=w:"+wt+"&d=docs/sub")
	if !hasLink(body, "p", "docs/sub/b.md") || !hasLink(body, "p", "docs/sub/c.md") || strings.Index(body, "b.md") > strings.Index(body, "c.md") {
		t.Errorf("sub index: %s", body)
	}
	_, body = get(t, repo, "/?s=b:main&d=.")
	if !hasLink(body, "p", "README.md") || !hasLink(body, "d", "docs") {
		t.Errorf("root index: %s", body)
	}
	if !strings.Contains(body, "article") {
		t.Error("index not in page template")
	}
}

func TestDirectoryIndexRejects(t *testing.T) {
	repo, wt := fixture(t)
	srv := srvFor(repo)
	key := "w:" + wt
	mk := func(d, ds string) string {
		return "/?" + url.Values{"s": {key}, "ss": {srv.sigSource(key)}, "d": {d}, "ds": {ds}}.Encode()
	}
	cases := map[string]struct {
		url  string
		want int
	}{
		"unsigned":         {"/?s=" + url.QueryEscape(key) + "&ss=" + srv.sigSource(key) + "&d=docs", 400},
		"forged":           {mk("docs", "AAAA"), 400},
		"doc sig":          {mk("docs", srv.sigDoc("docs")), 400},
		"source sig":       {mk("docs", srv.sigSource(key)), 400},
		"traversal":        {mk("../..", srv.sigDir("../..")), 404},
		"traversal in dir": {mk("docs/../..", srv.sigDir("docs/../..")), 404},
		"absolute":         {mk("/etc", srv.sigDir("/etc")), 404},
		"no docs dir":      {mk("src", srv.sigDir("src")), 404},
		"unslashed":        {mk("docs/", srv.sigDir("docs/")), 404},
	}
	for name, c := range cases {
		gitCalls.Store(0)
		fsReads.Store(0)
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, httptest.NewRequest("GET", c.url, nil))
		if rec.Code != c.want {
			t.Errorf("%s: %d, want %d", name, rec.Code, c.want)
		}
		if c.want == http.StatusBadRequest && (gitCalls.Load() != 0 || fsReads.Load() != 0) {
			t.Errorf("%s: work before verification", name)
		}
	}
	// a doc signature must not verify a doc path as directory either way round
	if c := reqCode(srv, "/?"+url.Values{"s": {key}, "ss": {srv.sigSource(key)}, "p": {"docs/a.md"}, "ps": {srv.sigDir("docs/a.md")}}.Encode()); c != 400 {
		t.Errorf("dir sig as doc sig: %d", c)
	}
}

func TestDirectoryIndexPlainDirectory(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "docs/x.md", "[up](../docs/)\n")
	write(t, dir, "README.md", "[d](docs)\n")
	key := "d:" + dir
	for _, u := range []string{"/?s=" + key + "&p=README.md", "/?s=" + key + "&p=docs/x.md"} {
		_, body := get(t, dir, u)
		if !hasLink(body, "d", "docs") {
			t.Fatalf("%s: dir link missing", u)
		}
	}
	code, body := get(t, dir, "/?s="+key+"&d=docs")
	if code != 200 || !hasLink(body, "p", "docs/x.md") {
		t.Errorf("%d %s", code, body)
	}
	if code, _ := get(t, dir, "/?s="+key+"&d=.."); code != 404 {
		t.Errorf("traversal: %d", code)
	}
}
