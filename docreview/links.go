package main

import (
	"net/url"
	"path"
	"slices"
	"strings"

	"github.com/yuin/goldmark/v2/ast"
	"github.com/yuin/goldmark/v2/text"
)

// dirEntries lists the immediate subdirectories and docs of dir (a slash path, "." for the root), both sorted.
// Directories are derived from the doc list, so only directories that hold docs exist and nothing leaves the root.
func dirEntries(docs []string, dir string) (subdirs, files []string) {
	prefix := dir + "/"
	if dir == "." {
		prefix = ""
	}
	for _, d := range docs {
		rest, ok := strings.CutPrefix(d, prefix)
		if !ok {
			continue
		}
		if seg, _, nested := strings.Cut(rest, "/"); nested {
			if sub := path.Join(dir, seg); !slices.Contains(subdirs, sub) {
				subdirs = append(subdirs, sub)
			}
		} else {
			files = append(files, d)
		}
	}
	slices.Sort(subdirs)
	slices.Sort(files)
	return
}

// rewriteLinks points relative links that resolve to another doc, or a directory holding docs, of the same source
// back at the viewer. Everything else (fragments, absolute URLs, non-docs, paths leaving the repo) is left alone.
func rewriteLinks(root ast.Node, src []byte, srcKey, srcSig, doc string, docs []string, docSig, dirSig func(string) string) {
	_ = ast.Walk(root, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		l, ok := n.(*ast.Link)
		if !ok || !entering {
			return ast.WalkContinue, nil
		}
		u, err := url.Parse(l.Destination.Value(src))
		if err != nil || u.Scheme != "" || u.Host != "" || u.Path == "" || strings.HasPrefix(u.Path, "/") {
			return ast.WalkContinue, nil
		}
		p := path.Join(path.Dir(doc), u.Path)
		q := url.Values{"s": {srcKey}, "ss": {srcSig}}
		if slices.Contains(docs, p) {
			q.Set("p", p)
			q.Set("ps", docSig(p))
		} else if sub, files := dirEntries(docs, p); len(sub)+len(files) > 0 {
			q.Set("d", p)
			q.Set("ds", dirSig(p))
		} else {
			return ast.WalkContinue, nil
		}
		dest := "/?" + q.Encode()
		if u.Fragment != "" {
			dest += "#" + url.PathEscape(u.Fragment)
		}
		l.Destination = text.NewSingleLineValueFromString(dest, text.IdentityDecoder)
		return ast.WalkContinue, nil
	})
}
