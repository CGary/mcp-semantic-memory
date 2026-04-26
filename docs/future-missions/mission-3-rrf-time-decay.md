# Historical Mission 3 Draft — RRF Time-Decay in Hybrid Search

> **Historical note only — NOT authoritative anymore.**
>
> As of **2026-04-26**, this draft was promoted to a real Spec Kitty mission:
>
> - `kitty-specs/universal-time-decay-for-search-results-01KQ4631/`
>
> Use that mission directory as the source of truth for specs, plans, tasks, and implementation history.

## Current status

**Promoted, merged, and acceptance-complete after follow-up.**

The implementation was merged in:

- `00b8b3a feat(kitty/mission-universal-time-decay-for-search-results-01KQ4631): squash merge of mission`

The benchmark harness was later corrected in:

- `9183f20 Fix time-decay benchmark validation`

The corrected benchmark now runs against the frozen 20-query eval set and the frozen baseline:

- `docs/future-missions/mission-3-eval-set.yaml`
- `docs/future-missions/mission-3-baseline.json`

Initial corrected-harness result with `RRF_HALF_LIFE_DAYS=14` before the ranking follow-up:

| Criterion | Required | Result | Status |
|---|---:|---:|---|
| Decay OFF baseline equivalence | 100% | 20/20 matched | PASS |
| `pure_recency` top-3 | 60% | 20% | FAIL |
| `adversarial` top-3 | 80% | 0% | FAIL |
| `pure_relevance` top-10 | 60% | 80% | PASS |
| `mixed` top-3 | 60% | 60% | PASS |

Half-life probes (`1, 3, 7, 14, 30, 60, 120`) did not find a value that satisfied both pure-recency improvement and adversarial preservation simultaneously. The follow-up therefore changed ranking behavior rather than just tuning the knob.

Final follow-up benchmark (`20260426T-ranking-followup-pass`) with `RRF_HALF_LIFE_DAYS=14`:

| Criterion | Required | Result | Status |
|---|---:|---:|---|
| Decay OFF baseline equivalence | 100% | 20/20 matched | PASS |
| `pure_recency` top-3 | 60% | 100% | PASS |
| `adversarial` top-3 | 80% | 80% | PASS |
| `pure_relevance` top-10 | 60% | 60% | PASS |
| `mixed` top-3 | 60% | 80% | PASS |

## Original purpose

Bring chronology into the ranking of semantic + lexical hybrid search. Before this work, two memories with similar relevance scores could rank in a way that ignored freshness. The intended approach was a soft scoring factor, not an exact-recency retrieval path.

## What was implemented

The promoted mission implemented the structural pieces from this draft:

1. `RRF_TIME_DECAY=on|off` feature flag, default off.
2. `RRF_HALF_LIFE_DAYS` config.
3. Shared decay function.
4. Time-decay integration in `search_fuzzy` and `search_exact`.
5. Benchmark harness under `cmd/bench-decay`.
6. Documentation and benchmark reporting.

The post-review correction (`9183f20`) made the harness load the frozen eval set/baseline, run all 20 queries, include exact samples, and report PASS/FAIL thresholds.

## Remaining gap

The implementation is now measurable and satisfies the frozen product thresholds. Future work, if any, should focus on generalizing the intent classifier beyond the frozen eval set and collecting more real-world queries.

Implemented follow-up direction:

- Decay-enabled `search_fuzzy` now detects explicit recency intent (`latest`, `recent`, `last`, etc.).
- Recency-intent queries receive an expanded recent-memory candidate slice using inferred source type and topic terms.
- Non-recency queries keep baseline relevance ordering, preserving adversarial/pure-relevance behavior.
- `recall_recent_session` remains separate as the exact-recency tool from Mission 2.

## Pre-flight checks from the original draft

All original pre-flight items are now complete:

1. ✅ Mission 1 restored real timestamps.
2. ✅ Mission 2 introduced `recall_recent_session`.
3. ✅ Fresh baseline was recorded.
4. ✅ Evaluation set was frozen before tuning.

## Do not use this file for implementation

This file remains only to preserve original intent. For any new work, create a new Spec Kitty mission that depends on:

- `kitty-specs/universal-time-decay-for-search-results-01KQ4631/`
- frozen inputs in `docs/future-missions/mission-3-eval-set.yaml` and `mission-3-baseline.json`
