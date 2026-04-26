# Mission 3 Follow-up Plan — Make Time-Decay Pass Acceptance

**Date**: 2026-04-26
**Depends on**: `kitty-specs/universal-time-decay-for-search-results-01KQ4631/`
**Frozen inputs**:
- `docs/future-missions/mission-3-eval-set.yaml`
- `docs/future-missions/mission-3-baseline.json`

## Problem statement

The Universal Time-Decay mission has the required infrastructure, but the current ranking formula does not meet acceptance thresholds.

Corrected benchmark evidence from commit `9183f20`:

| Criterion | Required | Observed at half-life 14 | Status |
|---|---:|---:|---|
| Decay OFF baseline equivalence | 100% | 20/20 matched | PASS |
| `pure_recency` top-3 | 60% | 20% | FAIL |
| `adversarial` top-3 | 80% | 0% | FAIL |
| `pure_relevance` top-10 | 60% | 80% | PASS |
| `mixed` top-3 | 60% | 60% | PASS |

Half-life probes (`1, 3, 7, 14, 30, 60, 120`) did not produce an acceptable tradeoff. More aggressive decay improves some recency cases but damages adversarial cases; weaker decay preserves adversarial cases better but does not sufficiently improve pure-recency queries.

## Goal

Create a follow-up Spec Kitty mission that changes ranking behavior enough to satisfy the frozen Mission 3 acceptance thresholds while preserving default-off byte equivalence.

## Non-goals

- Do not alter the frozen eval set or baseline in this follow-up.
- Do not remove `recall_recent_session`; it remains Mission 2's exact-recency path.
- Do not add storage schema changes unless absolutely necessary.
- Do not claim acceptance from ad-hoc cherry-picked queries; only the frozen harness counts.

## Recommended implementation strategy

### Phase 0 — Diagnose ranking failure by query

1. Run the corrected harness for the current default (`half-life=14`) and export per-query OFF/ON top-10.
2. For each failed query, capture:
   - expected winner id
   - expected winner age
   - expected winner source type/project
   - OFF rank, ON rank
   - top competing ids and ages
   - whether the expected winner is absent from candidate generation or only loses during reranking
3. Classify failures into:
   - candidate missing before rerank
   - candidate present but decay too weak
   - candidate present but relevance gap too large
   - adversarial over-demotion

**Exit criterion**: A short diagnostic table explains every failed frozen query.

### Phase 1 — Separate candidate generation from reranking

The current approach can only rerank what `search_fuzzy` already returns. For many pure-recency failures, the expected newest record may not be in the initial candidate pool.

Implement a decay-enabled candidate expansion path:

1. Keep decay OFF path exactly unchanged.
2. When decay is ON, fetch a larger lexical/vector candidate pool before final top-10 truncation.
3. Add an optional recency candidate slice for queries with recency language (`latest`, `recent`, `last`, `most recent`, `today`, `yesterday`).
4. Merge candidates before final scoring.

**Exit criterion**: Pure-recency expected winners appear in the candidate pool for at least 4/5 pure-recency queries.

### Phase 2 — Replace simple multiplicative decay with safer blended scoring

The multiplicative formula over-penalizes old-but-vital adversarial memories. Evaluate a blended formula such as:

```text
final_score = relevance_score * (1 - recency_weight) + normalized_recency_score * recency_weight
```

or for RRF-like positive scores:

```text
final_score = relevance_score + recency_boost(query_intent, age_days)
```

Guidelines:

- Keep the operator-facing half-life knob if possible.
- Add an internal cap on recency impact so high-relevance old records remain retrievable.
- Consider intent-sensitive weighting:
  - recency language present → stronger recency boost
  - adversarial/pure relevance style query → weaker recency boost
- Keep exact-search behavior consistent with fuzzy where possible.

**Exit criterion**: Harness result meets all required thresholds on the frozen eval set.

### Phase 3 — Codify acceptance in tests

1. Add a CI-safe test that runs the harness logic against a small deterministic fixture.
2. Add a slower/manual acceptance command for the full production corpus.
3. Ensure `cmd/bench-decay` exits non-zero or clearly marks FAIL when thresholds are not met.
4. Keep `RRF_TIME_DECAY=off` byte-equivalence test mandatory.

**Exit criterion**: It is impossible to merge a future ranking change with a misleading green benchmark report.

### Phase 4 — Documentation and rollout

1. Update README with the final scoring behavior.
2. Document how to interpret acceptance failures.
3. Preserve one passing benchmark report under `data/benchmarks/` or reference it in the mission acceptance artifacts.
4. Keep the default as `off` until acceptance passes and the operator explicitly opts in.

## Proposed work packages

### WP01 — Failure diagnostics and candidate-pool audit

Owned surfaces:
- `cmd/bench-decay/**`
- `data/benchmarks/**` or temporary run output only
- documentation under the new mission directory

Deliverables:
- per-query failure table
- candidate-missing vs scoring-loses classification

### WP02 — Candidate expansion for decay-enabled fuzzy search

Owned surfaces:
- `src/core/search/fuzzy.go`
- new/updated tests under `tests/modules/`

Deliverables:
- larger candidate pool only when decay is ON
- optional recency candidate slice for recency-intent queries
- unchanged decay-OFF behavior

### WP03 — Safer scoring formula and threshold tuning

Owned surfaces:
- `src/core/search/decay.go`
- `src/core/search/fuzzy.go`
- `src/core/search/exact` logic currently in `fuzzy.go`
- `tests/modules/search_decay_test.go`

Deliverables:
- scoring formula that passes frozen thresholds
- tests for adversarial preservation and recency promotion

### WP04 — Acceptance harness enforcement and docs

Owned surfaces:
- `cmd/bench-decay/**`
- `README.md`
- follow-up mission acceptance artifacts

Deliverables:
- benchmark reports PASS/FAIL clearly
- acceptance artifact from a passing run
- operator docs updated

## Recommended next command

Create a new Spec Kitty mission, not a direct patch against the historical future docs:

```bash
spec-kitty agent mission setup-spec --mission-type software-dev --title "Time-decay ranking acceptance follow-up"
```

The mission should depend on:

- `universal-time-decay-for-search-results-01KQ4631`
- frozen eval/baseline artifacts in this directory
