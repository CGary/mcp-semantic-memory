# Future Missions

This folder holds **frozen measurement inputs** for missions that have been promoted into Spec Kitty (`kitty-specs/`). All planning notes have been deleted after mission completion.

## Frozen inputs (active — referenced by `cmd/bench-decay`)

| File | Purpose |
|------|---------|
| `mission-3-eval-set.yaml` | 20-query frozen eval set for the time-decay benchmark harness |
| `mission-3-baseline.json` | Frozen decay-OFF baseline snapshot (corpus at commit `c81b9cff141f`, 2026-04-26) |

These are the default paths used by `cmd/bench-decay --eval` and `cmd/bench-decay --baseline`. Do not modify or delete them.

## Completed mission chain

All three missions are merged and accepted:

1. `kitty-specs/engram-legacy-cutover-and-corpus-restoration-01KQ2SJK/` — complete
2. `kitty-specs/recency-fast-path-for-session-recall-01KQ405N/` — complete
3. `kitty-specs/universal-time-decay-for-search-results-01KQ4631/` — merged; ranking follow-up PASS achieved

Final benchmark (`20260426T-ranking-followup-pass`, `RRF_HALF_LIFE_DAYS=14`):

| Criterion | Required | Result | Status |
|---|---:|---:|---|
| Decay OFF baseline equivalence | 100% | 20/20 | PASS |
| `pure_recency` top-3 | 60% | 100% | PASS |
| `adversarial` top-3 | 80% | 80% | PASS |
| `pure_relevance` top-10 | 60% | 60% | PASS |
| `mixed` top-3 | 60% | 80% | PASS |

## Ideas not yet promoted to a mission

- `ideas/cli-tool.md` — CLI binary `hsme-cli` to consume MCP tools from the terminal.
- `ideas/graph-cleanup-maintenance.md` — janitor job for graph cleanup and entity merging.
