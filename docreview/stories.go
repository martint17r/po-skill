package main

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/yuin/goldmark/v2/ast"
	extast "github.com/yuin/goldmark/v2/extension/ast"
	"github.com/yuin/goldmark/v2/text"
)

const epicsDoc = "docs/epics.md"

var (
	storyRe = regexp.MustCompile(`\bE\d+S\d+\b`)
	// epics.md story rows are table cells that start with a bold ID.
	storyRowRe = regexp.MustCompile(`(?m)^\|\s*\*\*(E\d+S\d+)\*\*`)
)

// storyIDs returns the story IDs defined in an epics.md source.
func storyIDs(epics []byte) map[string]bool {
	ids := map[string]bool{}
	for _, m := range storyRowRe.FindAllSubmatch(epics, -1) {
		ids[string(m[1])] = true
	}
	return ids
}

// storyBase is the link target of epics.md as seen from doc: "" on epics.md itself (same-page anchor), else a signed viewer URL.
func storyBase(srcKey, srcSig, doc string, docSig func(string) string) string {
	if doc == epicsDoc {
		return ""
	}
	return "/?" + url.Values{"s": {srcKey}, "ss": {srcSig}, "p": {epicsDoc}, "ps": {docSig(epicsDoc)}}.Encode()
}

// linkStories turns known story IDs in plain text into links to their row in epics.md (base, see storyBase) and, on
// epics.md itself, gives each story row cell an id. Code spans and existing links are left alone.
func linkStories(root ast.Node, src []byte, ids map[string]bool, base string, onEpics bool) {
	var texts []*ast.Text
	_ = ast.Walk(root, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n := n.(type) {
		case *ast.CodeSpan, *ast.Link, *ast.AutoLink:
			return ast.WalkSkipChildren, nil
		case *extast.TableCell:
			if s, ok := n.FirstChild().(*ast.Strong); ok && onEpics {
				if t, ok := s.FirstChild().(*ast.Text); ok && s.ChildCount() == 1 {
					if id := t.Value.Value(src); ids[id] {
						n.SetAttribute("id", text.NewMultiLineValueFromString(strings.ToLower(id), text.IdentityDecoder))
						return ast.WalkSkipChildren, nil
					}
				}
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
			if !ids[id] {
				continue
			}
			if m[0]+idx.Start > pos {
				t.Parent().InsertBefore(t, piece(pos, idx.Start+m[0]))
			}
			l := ast.NewLink(text.NewSingleLineValueFromString(base+"#"+strings.ToLower(id), text.IdentityDecoder))
			l.AppendChild(piece(idx.Start+m[0], idx.Start+m[1]))
			t.Parent().InsertBefore(t, l)
			pos = idx.Start + m[1]
		}
		if pos != idx.Start {
			t.Value = text.NewSingleLineValueFromIndex(text.NewIndex(pos, idx.Stop), dec)
		}
	}
}
