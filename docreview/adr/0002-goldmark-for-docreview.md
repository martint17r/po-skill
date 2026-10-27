# 2. Use goldmark in tools/docreview

Date: 2026-10-26

## Status

Accepted

## Context

`docreview` renders the repo's Markdown docs as HTML for review. The Go
standard library has no Markdown renderer.

## Decision

Use `github.com/yuin/goldmark/v2` (v1 until 2026-10-27) with its GFM parser and HTML renderer extensions, raw HTML off. It lives only
in the separate `docreview` module, so the service module does not depend on it.

## Consequences

- One extra dependency for a dev tool; it must stay actively maintained.
- Docs cannot inject raw HTML into the rendered page.
