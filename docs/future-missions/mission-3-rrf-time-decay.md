# Future Mission 3 — RRF Time-Decay in Hybrid Search

**Status**: Still future / not yet created in spec-kitty as of 2026-04-26.
**Depends on**:
- Mission 1 already exists historically: `kitty-specs/engram-legacy-cutover-and-corpus-restoration-01KQ2SJK/`
- Mission 2 now also exists: `kitty-specs/recency-fast-path-for-session-recall-01KQ405N/`

**Important**: This file is still only a draft note. If/when Mission 3 is promoted, the authoritative artifacts must live under `kitty-specs/`.

## Purpose

Bring chronology into the ranking of semantic + lexical hybrid search. Today, two memories with similar relevance scores can still rank in a way that ignores freshness. This mission would make recency a soft scoring factor, not an exact retrieval path.

## Source idea

`ideas/session-history-recency.md` — Solution 2 (RRF Time-Decay).

## Scope

### In scope

1. **Decay function**: multiplicative exponential half-life applied to the RRF score.
2. **Configurable half-life**: environment variable `RRF_HALF_LIFE_DAYS`.
3. **Feature flag**: `RRF_TIME_DECAY=on|off`, default `off` initially.
4. **A/B harness**: reproducible benchmark against a fixed query set.
5. **Documentation**: explain the knob, tradeoffs, and how to tune it.

### Out of scope

- Replacing RRF entirely
- Cross-encoder reranking / MMR / other ranking strategies
- Changes to Mission 2's exact-recency tool

## Pre-flight checks before promoting this mission

1. Confirm Mission 1 artifacts remain the authoritative restored-timestamp baseline.
2. Confirm Mission 2 exists and is the authoritative exact-recency fast path.
3. Record fresh baseline measurements from the current corpus.
4. Freeze an evaluation query set before tuning half-life values.

## Anti-confusion note

Mission 3 is the **only** mission in this folder that still remains genuinely future as of 2026-04-26.
