# 11. Sign docreview URL parameters with a typed HMAC

Date: 2026-10-26

## Status

Accepted

## Context

`docreview` passes the query values `s` (source key) and `p` (doc path) to `git` and the filesystem. They were checked against allowlists, but gosec (G702, G204, G703) rightly sees user input reaching `exec.Command` and `os.ReadFile`.

## Decision

- At start the server draws a 32-byte secret from `crypto/rand`, keeps it in memory only, never logs it.
- Every `s` and `p` the server emits carries a signature: `base64url(HMAC-SHA256(secret, "docreview/v1" 0x00 type 0x00 value))`, unpadded. Types: `branch`, `worktree`, `directory` (derived from the key prefix `b:`, `w:`, `d:`), `doc` and `dir` (a directory index, query values `d`/`ds`; only directories derived from the source's doc list exist).
- One signature per value in its own parameter: `ss` for `s`, `ps` for `p`, `ds` for `d`. The type is inside the MAC, so a signature never validates a value of another kind (branch for worktree, source for path, and so on); a combined signature would add nothing, since each value must verify anyway.
- Verified with `hmac.Equal` at the start of `/` and `/hash`, before any lookup, git call or file read. A missing or bad signature answers 400 `invalid link`, without saying which part failed. The allowlist checks stay as a second layer. A request without `s`/`p` is allowed and renders the default source and doc with freshly signed links.
- Signed: source dropdown options and doc list (as `data-sig`, copied into hidden `ss`/`ps` form fields on change), rewritten doc and directory links, story-ID links to `docs/epics.md` (type `doc`), directory-index entries, the `/hash` poll URL (from `data-ss`/`data-ps` of the article). The reload script treats a 400 like a gone doc and stops.
- G203 on `template.HTML(goldmark output)` is suppressed: raw HTML is off ([ADR 0002](./0002-goldmark-for-docreview.md)), links are escaped and signed. The same holds for the directory-index HTML built in `serve`: every name, path and query value goes through `html.EscapeString`, and the directories come only from the signed source's doc list.
- G705 on `io.WriteString` in `/hash` is suppressed with `#nosec`: it writes a hex fingerprint (sha256 or git blob id) as `text/plain`, only after `s` and `p` passed HMAC verification.
- G114 (missing HTTP server timeouts) is fixed, not suppressed: the server sets read-header,
  read, write and idle timeouts ([`main.go`](../main.go)).
- The remaining gosec findings cannot see the check, so `#nosec G702 G204` (`exec.Command`) and `#nosec G703` (`os.ReadFile`) name this ADR.

## Consequences

- Restarting the server invalidates old links and open tabs stop reloading; reload the page.
- The no-JavaScript `Show` button is gone: the form needs script to pick up signatures.
- Tests count git calls and filesystem reads to prove bad requests do no work.
