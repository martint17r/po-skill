# Architecture — <project>

<Deployables and planes, citing the tenets that shape them.> Decisions are recorded in [`adr/`](./adr/).

## <Main flow>

- **In:** <how data or requests arrive>
- **Storage:** <where state lives>

## Cross-cutting

- **Config:** <flags, env, files>
- **Secrets:** <where they live; never in repo, logs or artifacts>
- **Logging / Observability:** <logs, health, metrics>
- **Build/CI:** <the checks every PR must pass>
