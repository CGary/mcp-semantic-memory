# Future Mission 3 — RRF Time-Decay in Hybrid Search

**Status**: Drafted, NOT YET CREATED in spec-kitty.
**Depends on**: Mission 1 merged AND Mission 2 merged (or at least baseline measurements taken).
**Reason for deferral**: This mission tunes the search ranking core. The half-life parameter and the A/B comparison can only be specified meaningfully against a corpus that already has real timestamps and a working recency baseline.

## Purpose

Bring chronology into the ranking of semantic + lexical hybrid search. Today, two memories with similar relevance scores tie on score alone — the older one wins as often as the newer one. After this mission, recency acts as a tiebreaker (and a soft penalty on very old memories) so that when relevance is comparable, fresher memories surface first.

## Source idea

`ideas/session-history-recency.md` — Solution 2 ("RRF Time-Decay").

## Scope

### In scope

1. **Decay function**: Multiplicative exponential half-life applied to the RRF score. `score' = score * 0.5 ^ (age_days / HALF_LIFE_DAYS)`.
2. **Configurable half-life**: Environment variable `RRF_HALF_LIFE_DAYS` with a default value to be tuned during the mission. Initial proposal: 14 days.
3. **Feature flag**: Environment variable `RRF_TIME_DECAY` with values `on` / `off`, default `off` for the first release. Lets the operator A/B without redeploying.
4. **A/B harness**: A reproducible benchmark that runs a fixed query set against the production corpus with decay on vs decay off, reporting top-K rank changes for each query. Lives under `scripts/` or `tests/benchmarks/`.
5. **Documentation**: README section explaining the knob, what it does, and how to choose a half-life.

### Out of scope

- Per-query or per-source-type decay weighting (one knob is enough for v1).
- Replacing RRF with a different fusion algorithm.
- Re-ranking cross-encoder, MMR, or other post-retrieval techniques.
- Changes to `recall_recent_session` (Mission 2's tool) — that one is exact-recency, not decayed.

## Pre-flight checks before opening this mission

1. Mission 1 merged: `created_at` values are real, not migration timestamps.
2. Mission 2 merged: `recall_recent_session` available so the operator can compare "exact recency" vs "decayed relevance" head-to-head.
3. A frozen evaluation query set exists. At minimum 20 queries covering: pure recency intent, pure relevance intent, mixed intent, and adversarial cases (a very old highly-relevant memory that SHOULD NOT lose to a recent irrelevant one).
4. Baseline numbers recorded: for each query in the eval set, the rank of the "expected best result" with decay off.

## Likely Functional Requirements (preview, not authoritative)

- FR-001: `search_fuzzy` SHALL apply a multiplicative time-decay factor to the RRF score when `RRF_TIME_DECAY=on`.
- FR-002: The half-life SHALL be configurable via `RRF_HALF_LIFE_DAYS` (default TBD during specify).
- FR-003: When `RRF_TIME_DECAY=off`, the ranking SHALL be byte-identical to the pre-mission behavior.
- FR-004: The decay SHALL be computed against the memory's `created_at` (NOT `updated_at`).
- FR-005: Memories with `created_at` in the future relative to the query time SHALL be treated as zero-age (decay factor = 1.0). Defensive against clock skew or bad data.
- FR-006: A benchmark harness SHALL produce a side-by-side report for the eval query set with decay on vs decay off.

## Likely Non-Functional Requirements (preview)

- NFR-001: Search latency overhead ≤ 5% with decay enabled.
- NFR-002: On the eval set, ≥ 80% of "pure recency" queries surface the expected newest result in the top 3 with decay on. (Threshold tuned during specify against the actual baseline.)
- NFR-003: On the eval set, "very old high-relevance" queries do NOT lose their expected result from the top 5 — i.e., the decay must not be so aggressive that important architectural memories disappear.

## Likely Constraints (preview)

- C-001: The decay factor SHALL be applied at the chunk level before aggregation, not after — so a memory with multiple chunks of the same age does not get penalized N times.
- C-002: The default `RRF_TIME_DECAY=off` for the first release ensures zero behavioral change for existing users until they opt in.

## Open questions to resolve at specify time

- Q1: Initial default for `RRF_HALF_LIFE_DAYS` — 7, 14, or 30? Depends on the corpus's typical access pattern; needs a small experiment during specify.
- Q2: Should the decay be capped at a minimum factor (e.g., 0.1) to prevent very old memories from being effectively unreachable? Cap is safer; uncapped is purer.
- Q3: Should `search_exact` get the same treatment? Argument for: consistency. Argument against: exact search is often used for finding a known thing, where recency matters less. Tentative: leave `search_exact` untouched in v1.
- Q4: How to handle memories where `created_at` was restored from legacy (Mission 1) vs memories born in HSME post-cutover? Both should use `created_at` uniformly — that's the whole point of Mission 1.

## Effort estimate

Medium. ~5-7h:
- 1h: read RRF implementation in `src/core/search/fuzzy.go`, design the decay integration point.
- 1h: implement the decay + flag.
- 1h: implement the A/B benchmark harness.
- 1h: tune half-life against the eval set.
- 1h: tests (unit for the decay function, integration for end-to-end ranking).
- 1h: documentation + README.
- (Buffer): up to 1h for surprises.

## Risks and how the spec will mitigate them

1. **Aggressive decay buries vital architectural memories.** Mitigation: NFR-003 explicitly tests the "very old high-relevance" case. Long half-life default (14d+).
2. **Operator turns it on without measuring.** Mitigation: README spells out the A/B harness flow. Default off.
3. **Corpus has clock-skewed timestamps post-Mission-1.** Mitigation: FR-005 (future timestamps treated as zero-age).
4. **Performance regression at scale.** Mitigation: NFR-001 latency budget + benchmark.

## Why this is not Mission 1 or 2

Both predecessors are about correctness. This one is about quality. Quality work belongs after correctness, not alongside it. Mission 1 fixes broken data. Mission 2 ships a low-risk new tool. Mission 3 tunes a knob in the existing engine — and tuning a knob whose inputs were broken (pre-Mission-1) or whose comparator didn't exist (pre-Mission-2) would be flying blind.
