# docs/epics.md

One file, or an index plus one file per epic when it grows too long ([One file per epic](#one-file-per-epic)).

## Layout

```markdown
# Epics & Stories — <project>

<one paragraph: what the project does; links to tenets/architecture/contracts>

## Milestones
- **M1 — <name>** → <which requirement it covers>

## Milestone 1 — <name>

### Epic E1 — <goal>
**Why:** <the outcome this epic buys>

| Story | I want | Acceptance | Status |
|---|---|---|---|
| **E1S1** <short title> | <need, one sentence> | <testable criteria> | open |

## Cross-cutting
## Out of scope
## Open questions
```

- Epics are numbered `E<n>` across the whole file, in creation order; a milestone groups epics and
  may hold epics with any number. Mark an epic that sits outside its numeric neighbourhood with
  `*(M1)*` after the heading.
- The first cell of a story row starts with the bold ID, `**E1S1**`, followed by the title.
  docreview finds stories by exactly this pattern (`| **E1S1**`) to give each row an anchor
  (`#e1s1`) and link IDs elsewhere to it.
- Story IDs are `E<epic>S<n>` (`E1S1`) or, in a repo that already writes them so, `E<epic>-S<n>`
  (`E1-S1`). Use one form per repo; docreview and the docs-only check accept both.
- An assumption a story or milestone rests on is a blockquote (`> Assumption: …`) **and** a row in
  `docs/decisions-open.md`.

## One file per epic

When `epics.md` gets too long to read or review, split it:

- `docs/epics.md` keeps the intro, `## Milestones`, `## Cross-cutting`, `## Out of scope` and an
  epic index: one row per epic with its milestone and a relative link to its file.
- Each epic gets its own file whose name starts with `epic` (`docs/epics/epic-03-detection.md`,
  or the repo's existing directory, e.g. `docs/backlog/epic-03-detection-engine.md`). docreview
  treats every such file as a story doc.
- In an epic file a story is either a table row as above, or a section whose heading starts with
  the ID, with its `Status` as the first line:

  ```markdown
  ## E3-S14 — Accessibility statement

  **Status:** open
  ```

  docreview anchors the heading (`#e3-s14`) and links the ID elsewhere to it. Long specs
  (contract, data, acceptance criteria) live in the story's section.
- The `Status` values below apply to both forms. Each story has its `Status` in exactly one
  place; a repo that also lists statuses elsewhere (a summary table in `AGENTS.md`) names which
  one is authoritative and checks they agree.

## Acceptance

Write each criterion so a test or check can prove it: inputs, observable result, failure behaviour.
Name the decision (`D4`), ADR (linked) or tenet (`T3`) a criterion relies on. A criterion that a
later milestone or another story owns says so (`… (E3S2)`).

## Status column

Exactly one of:

| Value | When |
|---|---|
| `open` | nothing implemented |
| `in progress` ([PR link]) | a PR is open for it |
| `partial — <what is missing>` | some criteria proven, others not |
| `done` | every criterion covered by a test or check |
| `done (<milestone or story>: <part>)` | every clause done except parts `epics.md` assigns to a later milestone or another story |

Update it in the same PR that changes the story's implementation. Never mark `done` on code alone:
implemented means covered by a test or check.

## Changing stories

- **Add:** next free `S<n>` in the epic; never fill a gap left by a moved or deleted story.
- **Move:** the story gets the next free ID in its new epic; replace every old ID in the repo
  (docs, code comments, tests, ADRs) in the same change.
- **Split:** keep the original ID for the first part, give the rest new IDs, and cross-reference.
- **Drop:** move it to `## Out of scope` with the reason, keep the ID there.
