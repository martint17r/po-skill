package main

import (
	"net/url"
	"path"
	"regexp"
	"strings"

	"github.com/yuin/goldmark/v2/ast"
	extast "github.com/yuin/goldmark/v2/extension/ast"
	"github.com/yuin/goldmark/v2/text"
)

const epicsDoc = "docs/epics.md"

var (
	// Story IDs: E1S8, or E1-S8 as some repos write them.
	storyRe = regexp.MustCompile(`\bE\d+-?S\d+\b`)
	// epics.md story rows are table cells that start with a bold ID.
	storyRowRe = regexp.MustCompile(`(?m)^\|\s*\*\*(E\d+-?S\d+)\*\*`)
	// Backlog files define a story with a heading that starts with its ID ("## E3-S14 — Title").
	storyHeadRe = regexp.MustCompile(`(?m)^#{1,6}[ \t]+(E\d+-?S\d+)\b`)
)

// storyDoc reports whether doc may define stories: docs/epics.md, or any doc whose file name starts with "epic"
// (e.g. docs/backlog/epic-03-detection.md).
func storyDoc(doc string) bool {
	return strings.HasPrefix(path.Base(doc), "epic")
}

// storyIDs returns the story IDs a story doc defines, by bold table row or by heading.
func storyIDs(src []byte) []string {
	var ids []string
	for _, re := range []*regexp.Regexp{storyRowRe, storyHeadRe} {
		for _, m := range re.FindAllSubmatch(src, -1) {
			ids = append(ids, string(m[1]))
		}
	}
	return ids
}

// docURL is the signed viewer URL of doc in the current source.
func docURL(srcKey, srcSig, doc string, docSig func(string) string) string {
	return "/?" + url.Values{"s": {srcKey}, "ss": {srcSig}, "p": {doc}, "ps": {docSig(doc)}}.Encode()
}

// definedHere returns the story ID a defining node (a story row's bold text, whole; a heading, as its prefix)
// starts with, if ids places that story in doc.
func definedHere(n ast.Node, src []byte, ids map[string]string, doc string, whole bool) string {
	t, ok := n.FirstChild().(*ast.Text)
	if !ok {
		return ""
	}
	v := string(t.Value.Value(src))
	id := v
	if !whole {
		if id = storyRe.FindString(v); id == "" || !strings.HasPrefix(v, id) {
			return ""
		}
	}
	if def, ok := ids[id]; !ok || def != doc {
		return ""
	}
	return id
}

// linkStories turns known story IDs in plain text into links to the doc that defines them (ids: ID → doc; base gives
// a doc's link target, "" for the current one) and gives each story row cell or story heading of the current doc an
// id to land on. Code spans and existing links are left alone.
func linkStories(root ast.Node, src []byte, ids map[string]string, doc string, base func(string) string) {
	var texts []*ast.Text
	anchor := func(n ast.Node, id string) {
		n.SetAttribute("id", text.NewMultiLineValueFromString(strings.ToLower(id), text.IdentityDecoder))
	}
	_ = ast.Walk(root, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n := n.(type) {
		case *ast.CodeSpan, *ast.Link, *ast.AutoLink:
			return ast.WalkSkipChildren, nil
		case *extast.TableCell:
			if s, ok := n.FirstChild().(*ast.Strong); ok && s.ChildCount() == 1 {
				if id := definedHere(s, src, ids, doc, true); id != "" {
					anchor(n, id)
					return ast.WalkSkipChildren, nil
				}
			}
		case *ast.Heading:
			if id := definedHere(n, src, ids, doc, false); id != "" {
				anchor(n, id)
				return ast.WalkSkipChildren, nil
			}
		case *ast.Text:
			texts = append(texts, n)
		}
		return ast.WalkContinue, nil
	})
	// Pieces of a split Text keep CommonMark decoding; the parser's GFM decoder is a stateless text.NewDecoder().
	dec := text.NewDecoder()
	piece := func(start, stop int) *ast.Text {
		return ast.NewText(text.NewSingleLineValueFromIndex(text.NewIndex(start, stop), dec))
	}
	for _, t := range texts {
		if t.Value.IsOwned() { // not backed by source positions, so nothing to split
			continue
		}
		idx := t.Value.Index()
		val := src[idx.Start:idx.Stop]
		pos := idx.Start
		for _, m := range storyRe.FindAllIndex(val, -1) {
			id := string(val[m[0]:m[1]])
			def, ok := ids[id]
			if !ok {
				continue
			}
			if m[0]+idx.Start > pos {
				t.Parent().InsertBefore(t, piece(pos, idx.Start+m[0]))
			}
			l := ast.NewLink(text.NewSingleLineValueFromString(base(def)+"#"+strings.ToLower(id), text.IdentityDecoder))
			l.AppendChild(piece(idx.Start+m[0], idx.Start+m[1]))
			t.Parent().InsertBefore(t, l)
			pos = idx.Start + m[1]
		}
		if pos != idx.Start {
			t.Value = text.NewSingleLineValueFromIndex(text.NewIndex(pos, idx.Stop), dec)
		}
	}
}
