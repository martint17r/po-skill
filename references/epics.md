# docs/epics.md

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
- An assumption a story or milestone rests on is a blockquote (`> Assumption: …`) **and** a row in
  `docs/decisions-open.md`.

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
