---
name: po
description: Product-owner bookkeeping for a repo's docs — epics and stories in docs/epics.md or one file per epic (milestones, story IDs like E1S8 or E1-S8, acceptance criteria, the Status column), open decisions in docs/decisions-open.md (D-numbers, the open → answered — in progress → recorded flow), ADRs in docs/adr/, the design tenets in docs/tenets.md (T-numbers), the architecture overview in docs/architecture.md, and the docreview viewer that renders docs/ as HTML. Use when adding, splitting, moving or closing a story or epic, updating a story's Status after a code change, raising, answering or recording a decision, writing an ADR, adding or changing a tenet or the architecture, reviewing a change against the tenets, checking a docs-only change, or when the user asks to view or review the docs in a browser.
---

# PO: epics, stories, decisions, tenets, architecture, docreview

A repo that uses this skill keeps its plan in `docs/`:

| File | Holds | Format |
|---|---|---|
| `docs/epics.md` | milestones → epics → stories, each with a `Status`; when it grows too long, an index plus one `epic*.md` file per epic | [references/epics.md](references/epics.md) |
| `docs/decisions-open.md` | decisions still open or answered but not yet recorded | [references/decisions.md](references/decisions.md) |
| `docs/adr/NNNN-<slug>.md` | recorded decisions | [references/decisions.md](references/decisions.md#adrs) |
| `docs/tenets.md` | numbered design principles (`T<n>`) that override convenience | [references/tenets-architecture.md](references/tenets-architecture.md) |
| `docs/architecture.md` | how the system is built today, with links to its ADRs | [references/tenets-architecture.md](references/tenets-architecture.md#docsarchitecturemd) |

Missing files: start from [templates/](templates/). Read the reference for a file before you change it.

## Rules

1. **Status in the same change.** A PR that changes a story's implementation updates that story's
   `Status` in `docs/epics.md` or its epic file. Implemented means covered by a test or check.
2. **Answers first.** When the user answers a decision, mark its row `answered — in progress` and
   quote the answer **before doing anything else** (implementation, review, QA), so no answer is
   lost while the work runs.
3. **Record, then delete.** Once an outcome is recorded (ADR, story, tenet or `AGENTS.md`), delete
   its row from `docs/decisions-open.md` in that same change.
4. **Assumptions are decisions.** Whenever you assume something the user has not decided, add an
   `open` row with what you assumed and what it blocks.
5. **Stable IDs.** Story IDs are `E<epic>S<n>` (or `E<epic>-S<n>`, one form per repo), decision
   IDs `D<n>`, ADRs `NNNN`. Never reuse or
   renumber an ID; a story that moves to another epic gets a new ID there and every reference
   is updated (`grep -rn 'E1S9' .`).
6. **Link references.** Every ADR mention in `docs/` (`ADR 0007`, each number of `ADRs 0005–0007`)
   is a relative link to the ADR file, outside code blocks.
7. **Link repo files.** Every reference to a repo file outside `docs/` (sources, tests, testdata,
   policies, scripts, workflows, agent configs, e.g. `internal/ingest/ingest.go:65`,
   `tools/e2e/e2e.sh`) is a link to that file's page in the repo's web UI on its forge, for
   `<branch>` and `<path>` (directories: the forge's directory view), with the forge's line anchor
   when a line or range is cited, outside code blocks. `<branch>` is the branch the doc lives on:
   `main` (the default branch) for everything that is or will be merged into it, so links do not
   break when a feature branch is deleted. The visible text stays the path (with `:line` if cited).
   Links between docs stay relative; `AGENTS.md` and `README.md` count as docs here and are linked
   relatively.
8. **One PR per change.** Every change goes through its own pull request from a feature branch.
   Never commit to, merge into or push `main` (the default branch) without the user's explicit ask
   for that change; a passing QA or docs-only check is not that ask.

## Tenets and architecture

- **Before implementing a story,** read the story, then `docs/tenets.md` and `docs/architecture.md`.
- **A change that seems to contradict a tenet: stop and ask.** Do not work around it. Either the
  change is reshaped, or the user changes the tenet (record the question in
  `docs/decisions-open.md` meanwhile).
- **Review against the tenets** after each code change: for every tenet the change touches, check
  that it holds; report only real violations with tenet, `file:line`, what breaks and the smallest fix.
- **Keep `architecture.md` current** in the same change that alters what it describes (planes,
  transport, storage, config, secrets, CI checks). It summarises and links; the reasoning lives in ADRs.
- **Cite, do not repeat:** stories, ADRs and agent instructions refer to `T<n>` instead of restating
  the principle.

## Checking a docs-only change

Before a docs-only PR is opened or merged, check and report each problem with `file:line`:

- every statement is consistent with the other docs, the ADRs and the code on the branch;
- story IDs use the repo's one form (`E<epic>S<n>` or `E<epic>-S<n>`) and exist in `docs/epics.md`
  or an epic file;
- decision, ADR and tenet references resolve; relative links and anchors resolve;
  repo files outside `docs/` are forge links as rule 7 says;
- decided items are recorded and removed from `docs/decisions-open.md`; assumptions are listed there;
- every row there has `Status` `open` or `answered — in progress` with the answer quoted, no row
  contradicts the rest of the docs, and no row remains for an already recorded decision;
- every story's `Status` matches what the code and tests on the branch deliver.

## Viewing the docs: docreview

`docreview/` in this skill is a local web server that renders `docs/**.md`, `AGENTS.md` and
`README.md` as HTML (GFM, Mermaid, auto-reload), from the working directory, a worktree or a branch.
Story IDs in text link to the story's row or heading in `docs/epics.md` or its epic file. Run it from the skill directory:

```sh
cd <skill dir>/docreview && go run . --repo /path/to/repo   # http://127.0.0.1:8080
```

Run it in the background and give the user the URL. Details: [docreview/README.md](docreview/README.md).
