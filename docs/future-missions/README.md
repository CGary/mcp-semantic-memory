# Future Missions

This folder holds **planning notes for missions that may exist later than the current active mission flow**. These notes are not authoritative once a mission has already been created under `kitty-specs/`.

## Important anti-confusion rule

Before using any file in `docs/future-missions/`, always check whether the mission already exists in `kitty-specs/`.

- If a mission already exists in `kitty-specs/`, the **authoritative artifacts** are the ones under `kitty-specs/<mission-slug>/`.
- The files in `docs/future-missions/` are then only **historical scoping notes**.
- Do **not** continue planning from `docs/future-missions/` if a real spec-kitty mission already exists.

## Current status as of 2026-04-26

| Mission | Status | Authoritative location |
|--------|--------|------------------------|
| Mission 1 — Engram Legacy Cutover & Corpus Restoration | **Real mission exists, implemented/merged historically** | `kitty-specs/engram-legacy-cutover-and-corpus-restoration-01KQ2SJK/` |
| Mission 2 — Recency Fast Path for Session Recall | **Real mission exists** | `kitty-specs/recency-fast-path-for-session-recall-01KQ405N/` |
| Mission 3 — RRF Time-Decay in Hybrid Search | **Still future / not yet created** | `docs/future-missions/mission-3-rrf-time-decay.md` |

## Why this file exists

These notes are still useful for:
- preserving original scoping intent,
- documenting why later missions were split,
- helping future sessions understand sequencing.

But they are **not** the source of truth after a mission is promoted into `kitty-specs/`.

## In-flight / authoritative mission chain

1. `kitty-specs/engram-legacy-cutover-and-corpus-restoration-01KQ2SJK/`
2. `kitty-specs/recency-fast-path-for-session-recall-01KQ405N/`
3. future only: `docs/future-missions/mission-3-rrf-time-decay.md`

## Other ideas not yet promoted to mission status

These still live in `ideas/` and have no formal mission yet:

- `ideas/cli-tool.md` — CLI binary `hsme-cli` to consume MCP tools from the terminal.
- `ideas/graph-cleanup-maintenance.md` — janitor job for graph cleanup and entity merging.
