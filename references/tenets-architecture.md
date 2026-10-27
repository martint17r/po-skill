# Tenets and architecture

## docs/tenets.md

The design principles that override convenience: what must stay true however a story is built.
Few (about 5–15), stable, each checkable against code.

```markdown
# Tenets — <project>

Design principles that override convenience. When a change seems to contradict a
tenet, stop and ask.

---

## T1 — <the principle as one sentence>.
<what it means in this system, what it rules out, and how a reviewer can tell it is kept;
link the ADRs that implement or except it>
```

- IDs are `T<n>`, never reused or renumbered. A dropped tenet keeps its heading with
  `*(retired: <reason>)*`.
- A tenet states a lasting principle (`Time is explicit and business-tz aware`), not a choice of
  tool or a task. Tool choices are ADRs; work is stories. A tenet may list its accepted
  exceptions, each linked to the ADR that grants it (e.g. "stdlib first; exceptions: kong
  ([ADR 0001](…))").
- Write each so a reviewer can name its typical violation (e.g. `time.Now()` inside a derivation
  breaks T3). That list is what a tenet review checks.
- Add or change a tenet only on the user's decision (via `docs/decisions-open.md` if it is not
  settled yet), in its own change, and check the stories and ADRs that cite it.

## docs/architecture.md

A one-page overview of how the system is built **now**: enough for a new contributor or agent to
place a change. It summarises and links; it does not argue.

```markdown
# Architecture — <project>

<one paragraph: deployables and planes, citing the tenets that shape them (T6)>.
Decisions are recorded in [`adr/`](./adr/).

## <Main flow, e.g. Event transport>
- **In:** …
- **Storage:** … ([ADR 0004](./adr/0004-….md))

## Cross-cutting
- **Config:** …
- **Secrets:** … (never in repo, logs or artifacts)
- **Logging / Observability:** …
- **Build/CI:** the checks every PR must pass — the list QA runs.
```

- Each bullet names the mechanism and links the ADR or tenet behind it; reasons go in the ADR.
- `Build/CI` is the authoritative list of checks; QA and agents run exactly these.
- Update it in the same change that alters what it describes. A statement here that the code no
  longer matches is a docs bug the docs-only check reports.
- Diagrams are Mermaid blocks (docreview renders them).

## How they relate

| Question | Where it goes |
|---|---|
| What must always hold? | tenet |
| Which tool / design did we pick, and why? | ADR |
| How does it fit together today? | `architecture.md` |
| What gets built next, and when is it done? | story in `epics.md` |
| Not decided yet? | row in `decisions-open.md` |
