package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func run(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir,
		"-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false"}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func write(t *testing.T, dir, name, content string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// fixture: main has docs; branch "feature" has an extra edit; worktree "wt" has an uncommitted edit.
func fixture(t *testing.T) (repo, wt string) {
	t.Helper()
	repo = t.TempDir()
	run(t, repo, "init", "-b", "main")
	write(t, repo, "README.md", "# Readme\n")
	write(t, repo, "docs/a.md", "# Alpha\n\n| x | y |\n|---|---|\n| 1 | 2 |\n\n<script>bad()</script>\n\n```mermaid\ngraph TD\n  A-->B\n```\n")
	write(t, repo, "docs/sub/b.md", "# Beta\n")
	write(t, repo, "docs/notes.txt", "no")
	write(t, repo, "src/c.md", "# not a doc\n")
	run(t, repo, "add", ".")
	run(t, repo, "commit", "-m", "init")
	run(t, repo, "branch", "feature")
	run(t, repo, "checkout", "feature")
	write(t, repo, "docs/a.md", "# Alpha on feature\n")
	run(t, repo, "commit", "-am", "feature")
	run(t, repo, "checkout", "main")
	wt = filepath.Join(evalDir(t, t.TempDir()), "wt")
	run(t, repo, "worktree", "add", "-b", "wtbranch", wt)
	write(t, wt, "docs/a.md", "# Alpha uncommitted\n")
	return repo, wt
}

func evalDir(t *testing.T, d string) string {
	t.Helper()
	r, err := filepath.EvalSymlinks(d)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

var servers = map[string]Server{}

// srvFor returns one server (one secret) per repo, so links rendered by one request verify on the next.
func srvFor(repo string) Server {
	if s, ok := servers[repo]; ok {
		return s
	}
	servers[repo] = NewServer(repo)
	return servers[repo]
}

// signURL adds the signatures a real page would carry for the s and p in raw.
func signURL(srv Server, raw string) string {
	u, _ := url.Parse(raw)
	q := u.Query()
	if s := q.Get("s"); s != "" {
		typ := srcType(s)
		if typ == "" {
			typ = "branch"
		}
		q.Set("ss", srv.mac(typ, s))
	}
	if p := q.Get("p"); p != "" {
		q.Set("ps", srv.sigDoc(p))
	}
	if d := q.Get("d"); d != "" {
		q.Set("ds", srv.sigDir(d))
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func get(t *testing.T, repo, url string) (int, string) {
	t.Helper()
	srv := srvFor(repo)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest("GET", signURL(srv, url), nil))
	return rec.Code, rec.Body.String()
}

func TestSources(t *testing.T) {
	repo, wt := fixture(t)
	srcs, err := Sources(repo)
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	for _, s := range srcs {
		keys = append(keys, s.Key)
	}
	got := strings.Join(keys, ",")
	for _, want := range []string{"b:main", "b:feature", "b:wtbranch", "w:" + wt} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s in %s", want, got)
		}
	}
	if !strings.Contains(srcs[1].Label, "(wtbranch)") && !strings.Contains(srcs[0].Label, "(wtbranch)") {
		t.Errorf("labels lack branch name: %+v", srcs)
	}
	// the repo's own directory is listed once, first
	n := 0
	for _, s := range srcs {
		if sameDir(s.Dir, repo) {
			n++
		}
	}
	if n != 1 || !sameDir(srcs[0].Dir, repo) || strings.HasPrefix(srcs[0].Key, "d:") {
		t.Errorf("own dir not listed exactly once, first: %+v", srcs)
	}
	// from a linked worktree, that worktree comes first
	if srcs, _ := Sources(wt); !sameDir(srcs[0].Dir, wt) {
		t.Errorf("worktree not first: %+v", srcs)
	}
	// a subdirectory of a repo is not a worktree root: directory source first
	if srcs, _ := Sources(filepath.Join(repo, "docs")); !strings.HasPrefix(srcs[0].Key, "d:") {
		t.Errorf("subdir: %+v", srcs)
	}
}

func TestPlainDirectory(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "docs/x.md", "# Plain\n\n```mermaid\ngraph TD\n  A-->B\n```\n")
	write(t, dir, "README.md", "# R\n")
	srcs, err := Sources(dir)
	if err != nil || len(srcs) != 1 || !strings.HasPrefix(srcs[0].Key, "d:") || !strings.HasPrefix(srcs[0].Label, "directory: ") {
		t.Fatalf("%+v %v", srcs, err)
	}
	code, body := get(t, dir, "/?s="+srcs[0].Key+"&p=docs/x.md")
	if code != 200 || !strings.Contains(body, `<pre class="mermaid">graph TD`) || !strings.Contains(body, `value="docs/x.md"`) {
		t.Errorf("%d %s", code, body)
	}
	if code, body := get(t, dir, "/"); code != 200 || !strings.Contains(body, "<h1") || strings.Count(body, `<option value="d:`) != 1 {
		t.Errorf("default: %d", code)
	}
	if code, _ := get(t, dir, "/?s=b:main"); code != 400 {
		t.Errorf("branch key in plain dir: %d", code)
	}
	if code, _ := get(t, dir, "/?p=../x.md"); code != 404 {
		t.Errorf("traversal: %d", code)
	}
	// a relative --repo works too
	t.Chdir(dir)
	if srcs, err := Sources("."); err != nil || len(srcs) != 1 || !sameDir(srcs[0].Dir, dir) {
		t.Errorf("relative: %+v %v", srcs, err)
	}
}

func TestOtherGitErrorStillFails(t *testing.T) {
	dir := t.TempDir() // a broken .git file is a git error other than "not a git repository"
	write(t, dir, ".git", "garbage\n")
	if _, err := Sources(dir); err == nil {
		t.Error("want error")
	}
}

func TestGitMissingFallsBackToDirectory(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if srcs, err := Sources(t.TempDir()); err != nil || len(srcs) != 1 {
		t.Errorf("%+v %v", srcs, err)
	}
}

func TestBareRepoHasOnlyDirectorySource(t *testing.T) {
	bare := t.TempDir()
	run(t, bare, "init", "--bare")
	srcs, err := Sources(bare)
	if err != nil || len(srcs) != 1 || !strings.HasPrefix(srcs[0].Key, "d:") {
		t.Fatalf("got %v, %v", srcs, err)
	}
}

func TestUnreadableDirectoryIs500(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores permissions")
	}
	dir := t.TempDir()
	write(t, dir, "docs/x.md", "# X\n")
	if err := os.Chmod(filepath.Join(dir, "docs"), 0); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(filepath.Join(dir, "docs"), 0o755)
	if code, _ := get(t, dir, "/"); code != http.StatusInternalServerError {
		t.Errorf("code %d", code)
	}
}

func TestDocsAndRead(t *testing.T) {
	repo, wt := fixture(t)
	b := Source{Ref: "refs/heads/main"}
	docs, err := b.Docs(repo)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(docs, ",") != "README.md,docs/a.md,docs/sub/b.md" {
		t.Errorf("branch docs: %v", docs)
	}
	w := Source{Dir: wt}
	wdocs, err := w.Docs(repo)
	if err != nil || strings.Join(wdocs, ",") != strings.Join(docs, ",") {
		t.Errorf("worktree docs: %v %v", wdocs, err)
	}
	if c, _ := b.Read(repo, "docs/a.md"); !strings.Contains(string(c), "# Alpha\n") {
		t.Errorf("branch read: %s", c)
	}
	if c, _ := w.Read(repo, "docs/a.md"); !strings.Contains(string(c), "uncommitted") {
		t.Errorf("worktree read: %s", c)
	}
	if _, err := (Source{Ref: "refs/heads/nope"}).Docs(repo); err == nil {
		t.Error("want error for bad ref")
	}
	// removed worktree dir: no docs, no error
	if d, err := (Source{Dir: filepath.Join(wt, "gone")}).Docs(repo); err != nil || len(d) != 0 {
		t.Errorf("%v %v", d, err)
	}
	// unreadable dir surfaces an error
	if os.Geteuid() != 0 {
		if err := os.Chmod(filepath.Join(wt, "docs", "sub"), 0); err != nil {
			t.Fatal(err)
		}
		defer os.Chmod(filepath.Join(wt, "docs", "sub"), 0o755)
		if _, err := w.Docs(repo); err == nil {
			t.Error("want error for unreadable dir")
		}
	}
}

func TestIsDoc(t *testing.T) {
	for p, want := range map[string]bool{
		"docs/a.md": true, "README.md": true, "AGENTS.md": true, "docs/a.txt": false, "src/a.md": false,
		"docs/../x.md": false, "/etc/passwd.md": false, "": false, "CLAUDE.md": false,
	} {
		if isDoc(p) != want {
			t.Errorf("isDoc(%q) != %v", p, want)
		}
	}
}

func TestHandlerBranchRendersMarkdownAndMermaid(t *testing.T) {
	repo, _ := fixture(t)
	code, body := get(t, repo, "/?s=b:main&p=docs/a.md")
	if code != 200 {
		t.Fatalf("code %d", code)
	}
	for _, want := range []string{"<h1", "Alpha", "<table>", `<pre class="mermaid">graph TD`, "A--&gt;B",
		`value="b:feature"`, `value="b:wtbranch"`, "worktree: ", `value="docs/sub/b.md"`} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Contains(body, "<script>bad()") {
		t.Error("raw HTML must not render")
	}
}

func TestHandlerWorktreeShowsUncommitted(t *testing.T) {
	repo, wt := fixture(t)
	_, body := get(t, repo, "/?s=w:"+wt+"&p=docs/a.md")
	if !strings.Contains(body, "Alpha uncommitted") {
		t.Error("worktree edit not shown")
	}
	_, body = get(t, repo, "/?s=b:feature&p=docs/a.md")
	if !strings.Contains(body, "Alpha on feature") {
		t.Error("feature branch content not shown")
	}
}

func TestHandlerDefaults(t *testing.T) {
	repo, _ := fixture(t)
	code, body := get(t, repo, "/")
	if code != 200 || !strings.Contains(body, "Readme") {
		t.Errorf("default page: %d", code)
	}
	code, body = get(t, repo, "/style.css")
	if code != 200 || !strings.Contains(body, "prefers-color-scheme") {
		t.Errorf("css: %d", code)
	}
}

func TestHandlerRejects(t *testing.T) {
	repo, _ := fixture(t)
	for url, want := range map[string]int{
		"/?s=b:nope":                    400,
		"/?s=--upload-pack=x":           400,
		"/?s=b:main&p=../../etc/passwd": 404,
		"/?s=b:main&p=/etc/passwd":      404,
		"/?s=b:main&p=src/c.md":         404,
		"/?s=b:main&p=docs/notes.txt":   404,
		"/nothing":                      404,
	} {
		if code, _ := get(t, repo, url); code != want {
			t.Errorf("%s: got %d want %d", url, code, want)
		}
	}
}

func TestHandlerEmptySourceAndReadErrors(t *testing.T) {
	repo := t.TempDir()
	run(t, repo, "init", "-b", "main")
	write(t, repo, "x.txt", "x")
	run(t, repo, "add", ".")
	run(t, repo, "commit", "-m", "x")
	code, body := get(t, repo, "/")
	if code != 200 || !strings.Contains(body, "No docs") {
		t.Errorf("%d %s", code, body)
	}
	// worktree with a doc that is removed after listing is a read error
	repo2, wt := fixture(t)
	s := srvFor(repo2)
	os.Remove(filepath.Join(wt, "docs", "a.md"))
	rec := httptest.NewRecorder()
	s.serve(rec, httptest.NewRequest("GET", signURL(s, "/?s=w:"+wt+"&p=docs/a.md"), nil))
	if rec.Code != 404 {
		t.Errorf("removed doc: %d", rec.Code)
	}
}

func TestReadErrorIs500(t *testing.T) {
	repo, wt := fixture(t)
	// a directory named like a doc is not a regular file, so it is not listed; make one unreadable instead
	p := filepath.Join(wt, "docs", "a.md")
	if os.Geteuid() == 0 {
		t.Skip("root ignores permissions")
	}
	if err := os.Chmod(p, 0); err != nil {
		t.Fatal(err)
	}
	if code, _ := get(t, repo, "/?s=w:"+wt+"&p=docs/a.md"); code != 500 {
		t.Errorf("code %d", code)
	}
}

func TestNonEnglishLocaleStillFallsBack(t *testing.T) {
	t.Setenv("LANG", "de_DE.UTF-8")
	t.Setenv("LC_ALL", "de_DE.UTF-8")
	if srcs, err := Sources(t.TempDir()); err != nil || len(srcs) != 1 || !strings.HasPrefix(srcs[0].Key, "d:") {
		t.Errorf("%+v %v", srcs, err)
	}
}

func TestHash(t *testing.T) {
	repo, wt := fixture(t)
	wtURL := "/hash?s=w:" + wt + "&p=docs/a.md"
	code, h1 := get(t, repo, wtURL)
	if code != 200 || len(h1) != 16 {
		t.Fatalf("worktree hash: %d %q", code, h1)
	}
	if _, again := get(t, repo, wtURL); again != h1 {
		t.Error("hash not stable")
	}
	write(t, wt, "docs/a.md", "# Alpha edited again\n")
	if _, h2 := get(t, repo, wtURL); h2 == h1 {
		t.Error("worktree hash unchanged after edit")
	}
	if code, h := get(t, repo, "/hash?p=docs/a.md"); code != 200 || len(h) != 16 {
		t.Errorf("default source: %d %q", code, h)
	}

	rec := httptest.NewRecorder()
	srv := srvFor(repo)
	srv.Handler().ServeHTTP(rec, httptest.NewRequest("GET", signURL(srv, "/hash?s=b:main&p=docs/a.md"), nil))
	if ct, cc := rec.Header().Get("Content-Type"), rec.Header().Get("Cache-Control"); !strings.HasPrefix(ct, "text/plain") || cc != "no-store" {
		t.Errorf("headers %q %q", ct, cc)
	}
	_, b1 := get(t, repo, "/hash?s=b:main&p=docs/a.md")
	write(t, repo, "docs/a.md", "# Alpha committed\n")
	run(t, repo, "commit", "-am", "edit")
	if _, b2 := get(t, repo, "/hash?s=b:main&p=docs/a.md"); b2 == b1 || len(b2) != 16 {
		t.Errorf("branch hash %q -> %q", b1, b2)
	}

	for url, want := range map[string]int{
		"/hash?s=nope&p=docs/a.md":     400,
		"/hash?s=b:main&p=docs/x.md":   404,
		"/hash?s=b:main":               404,
		"/hash?s=b:main&p=src/c.md":    404,
		"/hash?s=b:main&p=../../etc/x": 404,
	} {
		if code, _ := get(t, repo, url); code != want {
			t.Errorf("%s: %d, want %d", url, code, want)
		}
	}
}

func TestHashDirInPlaceOfDocIs404(t *testing.T) {
	repo, wt := fixture(t)
	p := filepath.Join(wt, "docs/a.md")
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(p, 0o755); err != nil { // a directory is not a regular file, so not a doc
		t.Fatal(err)
	}
	if code, _ := get(t, repo, "/hash?s=w:"+wt+"&p=docs/a.md"); code != 404 {
		t.Errorf("dir in place of doc: %d", code)
	}
}

func TestPageReloadScriptIndependentOfMermaidImport(t *testing.T) {
	repo, _ := fixture(t)
	_, body := get(t, repo, "/?s=b:main&p=docs/a.md")
	parts := strings.Split(body, "<script")
	if len(parts) != 3 || !strings.HasPrefix(parts[1], ` type="module"`) ||
		!strings.Contains(parts[1], "import mermaid") || strings.Contains(parts[2], "import mermaid") ||
		!strings.Contains(parts[2], "/hash?") {
		t.Error("reload code must live in its own script, apart from the mermaid import")
	}
}

func TestDocLinksRewritten(t *testing.T) {
	repo, wt := fixture(t)
	write(t, repo, "docs/a.md", "[sib](./notes.md) [sub](./sub/b.md) [frag](./sub/b.md#top) [up](../README.md) [esc](../../x.md) [dir](./sub/) [src](../src/c.md) [abs](https://example.com/docs/a.md) [mail](mailto:a@b.c) [h](#top) [root](/docs/sub/b.md)\n")
	write(t, repo, "docs/notes.md", "# n\n")
	write(t, repo, "AGENTS.md", "[a](docs/a.md)\n")
	run(t, repo, "add", ".")
	run(t, repo, "commit", "-m", "links")
	_, body := get(t, repo, "/?s=b:main&p=docs/a.md")
	for _, p := range []string{"docs/notes.md", "docs/sub/b.md", "README.md"} {
		assertSignedLink(t, repo, body, "b:main", p)
	}
	if !strings.Contains(body, "#top") {
		t.Error("fragment lost")
	}
	for _, want := range []string{
		`href="../../x.md"`, `href="../src/c.md"`, `href="https://example.com/docs/a.md"`,
		`href="mailto:a@b.c"`, `href="#top"`, `href="/docs/sub/b.md"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %s", want)
		}
	}
	_, body = get(t, repo, "/?s=b:main&p=AGENTS.md")
	assertSignedLink(t, repo, body, "b:main", "docs/a.md")
	// directory source: the param is the current source's key
	write(t, wt, "docs/a.md", "[b](./sub/b.md)\n")
	_, body = get(t, repo, "/?s=w:"+wt+"&p=docs/a.md")
	assertSignedLink(t, repo, body, "w:"+wt, "docs/sub/b.md")
}
