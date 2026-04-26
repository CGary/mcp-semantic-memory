# Future Missions

This folder holds **drafts** of missions that are scoped but not yet created in spec-kitty. Each file captures the intent, scope, dependencies, and likely requirements so future sessions can pick them up without re-deriving the context.

## Why these aren't real spec-kitty missions yet

Each future mission depends on data, baselines, or measurements that only exist after a previous mission ships. Specifying them prematurely would lock in thresholds based on assumptions that haven't been validated. The drafts here are **planning notes**, not authoritative specs — they will be regenerated from a clean `/spec-kitty.specify` interview when their dependencies are met.

## Status

| File | Source idea | Depends on | Status |
|------|-------------|------------|--------|
| [mission-2-recency-fast-path.md](mission-2-recency-fast-path.md) | `ideas/session-history-recency.md` (Solution 1) | Mission 1 merged (real `created_at` values) | Drafted, not yet created |
| [mission-3-rrf-time-decay.md](mission-3-rrf-time-decay.md) | `ideas/session-history-recency.md` (Solution 2) | Mission 1 + Mission 2 merged, baseline measured | Drafted, not yet created |

## In-flight mission

**Mission 1** — `engram-legacy-cutover-and-corpus-restoration-01KQ2SJK` (active in spec-kitty, see `kitty-specs/`).

## Other ideas not yet promoted to mission status

These live in `ideas/` and have no scoped draft yet:

- `ideas/cli-tool.md` — CLI binary `hsme-cli` to consume the four MCP tools from the terminal. Independent of the recency stack; can be picked up in parallel.
- `ideas/graph-cleanup-maintenance.md` — Janitor job for entity merging and dirty-node pruning in the knowledge graph. Larger scope; benefits from a corpus that has been stable for a while, so naturally comes after the recency missions.
