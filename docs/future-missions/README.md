# Future Missions

This folder holds **historical planning notes and pre-flight artifacts** for missions that were planned before being promoted into Spec Kitty (`kitty-specs/`).

## Important anti-confusion rule

Before using any file in `docs/future-missions/`, always check whether the mission already exists in `kitty-specs/`.

- If a mission already exists in `kitty-specs/`, the **authoritative artifacts** are under `kitty-specs/<mission-slug>/`.
- Files in `docs/future-missions/` become **historical scoping notes / frozen measurement inputs**.
- Do **not** continue planning from `docs/future-missions/` if a real Spec Kitty mission already exists, unless the file explicitly says it is a frozen input.

## Current status as of 2026-04-26

| Mission / artifact | Status | Authoritative location / notes |
|--------|--------|------------------------|
| Mission 1 — Engram Legacy Cutover & Corpus Restoration | **Implemented / merged** | `kitty-specs/engram-legacy-cutover-and-corpus-restoration-01KQ2SJK/` |
| Mission 2 — Recency Fast Path for Session Recall | **Implemented / accepted** | `kitty-specs/recency-fast-path-for-session-recall-01KQ405N/` |
| Mission 3 — Universal Time-Decay for Search Results | **Promoted, implemented structurally, but acceptance failed** | `kitty-specs/universal-time-decay-for-search-results-01KQ4631/` |
| Mission 3 pre-flight eval set | **Frozen input** | `docs/future-missions/mission-3-eval-set.yaml` |
| Mission 3 pre-flight baseline | **Frozen input** | `docs/future-missions/mission-3-baseline.json`, `mission-3-baseline.md` |
| Mission 3 original draft | **Historical note only** | `docs/future-missions/mission-3-rrf-time-decay.md` |
| Mission 3 follow-up plan | **Pending / not yet promoted** | `docs/future-missions/mission-3-follow-up-plan.md` |

## Mission 3 acceptance status

Mission 3 is **not future anymore**. It was promoted to:

- `kitty-specs/universal-time-decay-for-search-results-01KQ4631/`

and merged in commit:

- `00b8b3a feat(kitty/mission-universal-time-decay-for-search-results-01KQ4631): squash merge of mission`

A later corrective commit made the benchmark harness honest and comparable with the frozen eval artifacts:

- `9183f20 Fix time-decay benchmark validation`

After that correction, the benchmark shows:

- Decay OFF baseline equivalence: **PASS** (`20/20` frozen queries matched baseline)
- Overall acceptance: **FAIL**
  - `pure_recency` top-3: `20%`, required `60%`
  - `adversarial` top-3: `0%`, required `80%`
  - `pure_relevance` top-10: `80%`, required `60%` — PASS
  - `mixed` top-3: `60%`, required `60%` — PASS

Therefore, the code/harness exists, but Mission 3 still needs a follow-up ranking mission before the product requirement can be considered complete.

## In-flight / authoritative mission chain

1. `kitty-specs/engram-legacy-cutover-and-corpus-restoration-01KQ2SJK/` — complete
2. `kitty-specs/recency-fast-path-for-session-recall-01KQ405N/` — complete
3. `kitty-specs/universal-time-decay-for-search-results-01KQ4631/` — merged but acceptance-failing; requires follow-up

## Other ideas not yet promoted to mission status

These still live in `ideas/` and have no formal mission yet:

- `ideas/cli-tool.md` — CLI binary `hsme-cli` to consume MCP tools from the terminal.
- `ideas/graph-cleanup-maintenance.md` — janitor job for graph cleanup and entity merging.

## Recommended next mission

Promote `docs/future-missions/mission-3-follow-up-plan.md` into a new Spec Kitty mission focused on ranking-quality acceptance. The existing Universal Time-Decay mission should be treated as infrastructure plus failed acceptance evidence, not as a complete product outcome.
