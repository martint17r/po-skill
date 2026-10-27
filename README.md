# po-skill

An [Agent Skill](https://agentskills.io) for product-owner bookkeeping in a repo's `docs/`:
epics and stories with a `Status` column (`docs/epics.md`), open decisions
(`docs/decisions-open.md`), ADRs (`docs/adr/`), and **docreview**, a local viewer that renders the
docs as HTML. Works with Claude Code and pi; the skill itself is [`SKILL.md`](SKILL.md).

## Install

The skill directory must be named `po`.

```sh
# Claude Code (all projects) — or <repo>/.claude/skills/po for one project
git clone https://github.com/martint17r/po-skill.git ~/.claude/skills/po

# pi (all projects) — or <repo>/.pi/skills/po for one project
git clone https://github.com/martint17r/po-skill.git ~/.pi/agent/skills/po
```

Using both? Clone once and symlink the other path to it. Update with `git -C <dir> pull`.

Then add [`templates/AGENTS-snippet.md`](templates/AGENTS-snippet.md) to the repo's `AGENTS.md`
(or `CLAUDE.md`) so agents load the skill when they touch the plan, and start missing files from
[`templates/`](templates/).

## Layout

| Path | What |
|---|---|
| [`SKILL.md`](SKILL.md) | rules and workflow the agent follows |
| [`references/`](references/) | formats of `epics.md`, `decisions-open.md` and ADRs |
| [`templates/`](templates/) | starting files for a new repo |
| [`docreview/`](docreview/) | the doc viewer (Go); `cd docreview && go run . --repo <repo>` |

docreview needs Go 1.26+ and `git`; Mermaid and fonts load from the internet.
