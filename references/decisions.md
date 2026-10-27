# Decisions

## docs/decisions-open.md

Lists every decision that is still open or answered but not yet recorded. When none are left the
table is replaced by the line `No open decisions.`

```markdown
| ID | Status | Decision | Options | Assumed until decided | Blocks |
|---|---|---|---|---|---|
| D13 | open | <the question> | <options, with the recommended one first> | <what the docs/code assume meanwhile> | E5S0 |
```

- IDs are `D<n>`, increasing, never reused. Sub-items use `D11.2`.
- `Blocks` names stories (`E5S0`), milestones or a phase (`E4S2 go-live`).
- A repo may keep a second table (e.g. questions for another team) with the same columns.

### Lifecycle

1. **Raise** — when you need or assume something the user has not decided, add an `open` row in
   the same change that makes the assumption.
2. **Answer** — the moment the user answers, before any other work, change `Status` to
   `answered — in progress: <the answer, quoted>` and commit that on its own
   (e.g. `docs: D20 answered — in progress`).
3. **Record** — write the outcome where it belongs: an ADR (technical choice with trade-offs), a
   story (scope or acceptance), a tenet (lasting principle) or `AGENTS.md` (how agents work).
   Delete the row in the same change (e.g. `docs: record D20 — …`).

A row with `open` must not have an answer; a row the docs already settle must not exist.

## ADRs

`docs/adr/NNNN-<kebab-slug>.md`, numbered from `0001`, never renumbered.

```markdown
# N. <Decision as a short imperative>

Date: YYYY-MM-DD

## Status

Accepted            (or: Superseded by [ADR 00NN](./00NN-….md))

## Context

<forces, constraints, the decision ID it settles (D20)>

## Decision

<what we do, specific enough to check code against>

## Consequences

<what gets easier, what gets harder, what follow-up it needs>
```

To change a decision, write a new ADR and set the old one to `Superseded by …`; do not rewrite an
accepted ADR's decision. Every `ADR NNNN` mention in `docs/` is a relative link to its file.
