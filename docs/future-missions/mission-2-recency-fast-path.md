# Future Mission 2 — Recency-Aware Fast Path

> **Historical note only — NOT authoritative anymore.**
>
> As of **2026-04-26**, this mission was already promoted to a real Spec Kitty mission:
>
> - `kitty-specs/recency-fast-path-for-session-recall-01KQ405N/`
>
> Use the artifacts under `kitty-specs/recency-fast-path-for-session-recall-01KQ405N/` as the source of truth.
> This file remains only to preserve the original pre-spec scoping note.

**Historical status**: Drafted before promotion to a real mission.
**Current authoritative mission**: `recency-fast-path-for-session-recall-01KQ405N`
**Authoritative location**: `kitty-specs/recency-fast-path-for-session-recall-01KQ405N/`

## Original purpose

Solve the recency gap in `search_fuzzy`: semantic search prioritizes relevance over chronology, so when an agent asks "what did we do in the last session?" it can get results from weeks ago ranked above the actual latest session. The promoted real mission introduced a dedicated fast path rather than modifying RRF directly.

## Historical dependency note

This mission depended on Mission 1 restoring real `created_at` values and `project` metadata.

## What to do now

If you need to continue this work, do **not** use this file for planning.
Go to:

- `kitty-specs/recency-fast-path-for-session-recall-01KQ405N/spec.md`
- `kitty-specs/recency-fast-path-for-session-recall-01KQ405N/plan.md`
- `kitty-specs/recency-fast-path-for-session-recall-01KQ405N/tasks/` (when materialized)
