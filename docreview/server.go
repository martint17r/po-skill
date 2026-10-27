package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"embed"
	"encoding/base64"
	"fmt"
	"html"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/yuin/goldmark/v2/extension"
	"github.com/yuin/goldmark/v2/parser"
	mdhtml "github.com/yuin/goldmark/v2/renderer/html"
)

//go:embed web/*
var web embed.FS

var (
	page       = template.Must(template.ParseFS(web, "web/page.html"))
	mdParser   = parser.New(parser.WithExtensions(extension.GFMParser))
	mdRenderer = mdhtml.New(mdhtml.WithExtensions(extension.GFMHTMLRenderer)) // raw HTML stays off
	// goldmark emits fenced mermaid blocks as <code class="language-mermaid">; mermaid.js wants <pre class="mermaid">.
	mermaidRe = regexp.MustCompile(`(?s)<pre><code class="language-mermaid">(.*?)</code></pre>`)
)

// Server holds the repo and a per-process random secret that signs every source key and doc path it emits (ADR 0011).
type Server struct {
	Repo   string
	secret []byte
}

func NewServer(repo string) Server {
	secret := make([]byte, 32)
	_, _ = rand.Read(secret) // never fails since Go 1.24
	return Server{Repo: repo, secret: secret}
}

// mac signs value v as kind typ: HMAC-SHA256 over "docreview/v1" 0x00 typ 0x00 v, base64url unpadded.
// The type is part of the message, so a signature for one kind of value never verifies for another.
func (s Server) mac(typ, v string) string {
	m := hmac.New(sha256.New, s.secret)
	m.Write([]byte("docreview/v1\x00" + typ + "\x00" + v))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

// srcType names the kind of a source key; "" for anything this server never emits.
func srcType(key string) string {
	switch {
	case strings.HasPrefix(key, "b:"):
		return "branch"
	case strings.HasPrefix(key, "w:"):
		return "worktree"
	case strings.HasPrefix(key, "d:"):
		return "directory"
	}
	return ""
}

func (s Server) sigSource(key string) string { return s.mac(srcType(key), key) }
func (s Server) sigDoc(p string) string      { return s.mac("doc", p) }
func (s Server) sigDir(p string) string      { return s.mac("dir", p) }

// verified checks the signatures of s and p (when present) before anything is looked up, run or read.
func (s Server) verified(r *http.Request) bool {
	q := r.URL.Query()
	if k := q.Get("s"); k != "" {
		t := srcType(k)
		if t == "" || !hmac.Equal([]byte(q.Get("ss")), []byte(s.mac(t, k))) {
			return false
		}
	}
	if p := q.Get("p"); p != "" && !hmac.Equal([]byte(q.Get("ps")), []byte(s.sigDoc(p))) {
		return false
	}
	if d := q.Get("d"); d != "" && !hmac.Equal([]byte(q.Get("ds")), []byte(s.sigDir(d))) {
		return false
	}
	return true
}

type srcOpt struct {
	Source
	Sig string
}

type docOpt struct{ Path, Sig string }

type view struct {
	Sources []srcOpt
	Cur     Source
	CurSig  string
	Docs    []docOpt
	Doc     string
	DocSig  string
	Body    template.HTML
	Error   string
}

func (s Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.serve)
	mux.HandleFunc("GET /hash", s.hash)
	mux.HandleFunc("GET /style.css", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFileFS(w, r, web, "web/style.css")
	})
	return mux
}

// lookup resolves the "s" query value against the discovered sources and lists that source's docs.
// On failure it has already written the error response. Nothing user-supplied reaches git or the filesystem unchecked.
func (s Server) lookup(w http.ResponseWriter, r *http.Request) (srcs []Source, cur Source, docs []string, ok bool) {
	if !s.verified(r) {
		http.Error(w, "invalid link", http.StatusBadRequest)
		return
	}
	srcs, err := Sources(s.Repo)
	if err != nil || len(srcs) == 0 {
		http.Error(w, "no branches or worktrees found", http.StatusInternalServerError)
		return
	}
	cur = srcs[0]
	if k := r.URL.Query().Get("s"); k != "" {
		i := slices.IndexFunc(srcs, func(x Source) bool { return x.Key == k })
		if i < 0 {
			http.Error(w, "unknown source", http.StatusBadRequest)
			return
		}
		cur = srcs[i]
	}
	docs, err = cur.Docs(s.Repo)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	return srcs, cur, docs, true
}

// hash serves a short content fingerprint of one doc; the page polls it to know when to reload.
func (s Server) hash(w http.ResponseWriter, r *http.Request) {
	_, cur, docs, ok := s.lookup(w, r)
	if !ok {
		return
	}
	p := r.URL.Query().Get("p")
	if !slices.Contains(docs, p) {
		http.Error(w, "unknown doc", http.StatusNotFound)
		return
	}
	var id string
	if cur.Ref != "" {
		out, err := git(s.Repo, "rev-parse", cur.Ref+":"+p)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		id = strings.TrimSpace(string(out))
	} else {
		src, err := cur.Read(s.Repo, p)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		id = fmt.Sprintf("%x", sha256.Sum256(src))
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = io.WriteString(w, id[:16]) // #nosec G705 -- writes a hex fingerprint (sha256 or git blob id) as text/plain, only after s/p passed HMAC verification (ADR 0011)
}

// li writes one index entry linking to a signed doc ("p") or directory ("d").
func li(b *strings.Builder, key, keySig, param, v, sig, label string) {
	q := url.Values{"s": {key}, "ss": {keySig}, param: {v}, param + "s": {sig}}
	b.WriteString(`<li><a href="/?` + html.EscapeString(q.Encode()) + `">` + html.EscapeString(label) + "</a></li>\n")
}

func (s Server) serve(w http.ResponseWriter, r *http.Request) {
	srcs, cur, docs, ok := s.lookup(w, r)
	if !ok {
		return
	}
	v := view{Cur: cur, CurSig: s.sigSource(cur.Key)}
	for _, x := range srcs {
		v.Sources = append(v.Sources, srcOpt{x, s.sigSource(x.Key)})
	}
	for _, d := range docs {
		v.Docs = append(v.Docs, docOpt{d, s.sigDoc(d)})
	}
	if d := r.URL.Query().Get("d"); d != "" {
		sub, files := dirEntries(docs, d)
		if len(sub)+len(files) == 0 {
			http.Error(w, "unknown directory", http.StatusNotFound)
			return
		}
		var b strings.Builder
		b.WriteString("<h1>" + html.EscapeString(d) + "/</h1>\n<ul>\n")
		for _, e := range sub {
			li(&b, cur.Key, v.CurSig, "d", e, s.sigDir(e), e+"/")
		}
		for _, e := range files {
			li(&b, cur.Key, v.CurSig, "p", e, s.sigDoc(e), path.Base(e))
		}
		b.WriteString("</ul>")
		v.Body = template.HTML(b.String()) // #nosec G203 -- built from HMAC-verified dir and doc paths, every value HTML-escaped (ADR 0011)
	} else if p := r.URL.Query().Get("p"); p != "" || len(docs) > 0 {
		if p == "" {
			p = docs[0]
		}
		if !slices.Contains(docs, p) {
			http.Error(w, "unknown doc", http.StatusNotFound)
			return
		}
		src, err := cur.Read(s.Repo, p)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		var buf bytes.Buffer
		tree := mdParser.Parse(src)
		rewriteLinks(tree, src, cur.Key, v.CurSig, p, docs, s.sigDoc, s.sigDir)
		ids := map[string]string{} // story ID → the doc that defines it; the first definition wins
		for _, d := range docs {
			if !storyDoc(d) {
				continue
			}
			b, err := cur.Read(s.Repo, d)
			if err != nil {
				slog.Warn("story links skipped: story doc unreadable", "source", cur.Key, "doc", d, "error", err)
				continue
			}
			for _, id := range storyIDs(b) {
				if _, dup := ids[id]; !dup {
					ids[id] = d
				}
			}
		}
		if len(ids) > 0 {
			linkStories(tree, src, ids, p, func(d string) string {
				if d == p {
					return ""
				}
				return docURL(cur.Key, v.CurSig, d, s.sigDoc)
			})
		}
		if err := mdRenderer.Render(&buf, src, tree); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		v.Doc, v.DocSig = p, s.sigDoc(p)
		v.Body = template.HTML(mermaidRe.ReplaceAll(buf.Bytes(), []byte(`<pre class="mermaid">$1</pre>`))) // #nosec G203 -- goldmark renders with raw HTML disabled (ADR 0002); doc links are rewritten to escaped, signed URLs (ADR 0011)
	} else {
		v.Error = "No docs in this source."
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	_ = page.Execute(w, v)
}
