# docreview

Local web server that renders the repo's Markdown docs as HTML for review, including Mermaid diagrams.

## Run

```sh
cd docreview
go run . --repo /path/to/repo         # http://127.0.0.1:8080
```

| Flag | Env | Default | Meaning |
|---|---|---|---|
| `--repo` | `DOCREVIEW_REPO` | `.` | Directory to read; may be a git repository or a plain directory |
| `--addr` | `DOCREVIEW_ADDR` | `127.0.0.1:8080` | Listen address; keep it on loopback |

## What it shows

- Docs: every `.md` under `docs/`, plus `AGENTS.md` and `README.md` at the repo root.
- Top bar: a doc list and a source dropdown.
- Source dropdown, first entry and default is the `--repo` directory itself (`directory: <path>`, read from disk; it works without git and is not listed twice if it is a worktree). Further sources come from `git worktree list` and `git for-each-ref`:
  - `worktree: <dir>` reads files from disk, so uncommitted edits show.
  - `branch: <name>` reads committed content via `git ls-tree` and `git show`.
- Mermaid blocks render client-side with mermaid.js 11.4.1 from jsDelivr; fonts come from Google Fonts. Both need internet access.

- Links to a directory that holds docs (`./adr/`, `docs`, `../`) open an index of its docs and subdirectories, in the same source. Links to other directories are left as they are.

- Story IDs (`E1S8`) in plain text link to that story's row in the rendered `docs/epics.md` of the same source (signed like other doc links; a plain `#e1s8` on `epics.md` itself). Each story row there gets an `id` (`e1s8`) to land on. IDs in code, in existing links, or not defined in that source's `epics.md` stay plain text.

Only paths listed for the selected source are served. The page auto-reloads: while the tab is visible it polls `/hash` (sha256 of the file, or the git blob id for branches) once a second and swaps in the new content, keeping the scroll position, when it changes. It stops if the doc disappears. No auth, editing or search. Every source key, doc path and directory path in a URL is signed with an HMAC under a random per-start secret ([ADR 0011](adr/0011-hmac-signed-parameters-in-docreview.md)); links from before a restart answer 400.
