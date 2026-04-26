# Brief: Pre-flight Work Before `/spec-kitty.specify` for Mission 3

**Status**: Complete — historical pre-flight brief only.
**Created**: 2026-04-26
**Audience**: Whatever agent picks this up — assume zero context from prior conversations.
**Authoritative mission**: `kitty-specs/universal-time-decay-for-search-results-01KQ4631/`. Original draft: `docs/future-missions/mission-3-rrf-time-decay.md`.

---

## Why this brief exists

Mission 3 — RRF Time-Decay in Hybrid Search — has a draft in `docs/future-missions/mission-3-rrf-time-decay.md`. The draft itself lists four pre-flight checks that must pass before promoting it to a real `kitty-specs/` mission via `/spec-kitty.specify`. The first two pre-flight items are already satisfied:

1. ✅ Mission 1 (`engram-legacy-cutover-and-corpus-restoration-01KQ2SJK`) is merged AND executed against production. Real `created_at` values exist for the entire corpus (rango 2026-04-04 → ~2026-04-26).
2. ✅ Mission 2 (`recency-fast-path-for-session-recall-01KQ405N`) is merged. The `recall_recent_session` MCP tool exists and works.

The last two pre-flight items are now also complete:

3. ✅ Fresh baseline measurements were recorded from the then-current corpus.
4. ✅ The evaluation query set was frozen before tuning half-life values.

**Why these matter**: Mission 3's likely NFRs include thresholds like "≥80% of pure-recency queries surface the expected newest result in the top 3" and "very-old high-relevance memories do NOT disappear from the top 5 with decay enabled". Those numbers are meaningless without:
- A frozen set of representative queries to evaluate against (so we don't tune the half-life by cherry-picking).
- A "before" measurement against today's `search_fuzzy` so the "after" has something to beat.

This brief succeeded in preventing invented thresholds: the promoted Mission 3 used the frozen eval set and baseline. The later corrected benchmark showed the structural implementation exists but does not satisfy acceptance thresholds, so the remaining work is a follow-up ranking-quality mission.

---

## Goal of this brief — completed

Produce two artifacts that the next operator will reference inside Mission 3's `/spec-kitty.specify` discovery interview and inside Mission 3's `research.md`:

1. A frozen evaluation query set: `docs/future-missions/mission-3-eval-set.yaml`.
2. A baseline measurement report: `docs/future-missions/mission-3-baseline.json` plus a human-readable companion `docs/future-missions/mission-3-baseline.md`.

Both files were committed to `main`. This brief should no longer be executed; it remains as historical context for how the Mission 3 frozen artifacts were produced.

---

## Out of scope — DO NOT do these

- ❌ Do NOT run `/spec-kitty.specify` for Mission 3. That is the NEXT step after this brief is done. Stop after the two artifacts are committed.
- ❌ Do NOT modify any source file under `src/`, `cmd/`, `tests/`, `scripts/`, or `kitty-specs/`.
- ❌ Do NOT change the schema. Do NOT touch `src/storage/sqlite/db.go`.
- ❌ Do NOT modify `CLAUDE.md`.
- ❌ Do NOT run the migrator (`./migrate-legacy`). Mission 1 is already done.
- ❌ Do NOT propose changes to the RRF algorithm itself. That is Mission 3's actual implementation, not this brief.
- ❌ Do NOT consult external benchmarks or web sources to invent thresholds. The whole point is to measure THIS corpus.

---

## Step 1 — Define and freeze the evaluation query set

### File to create: `docs/future-missions/mission-3-eval-set.yaml`

The set MUST contain at least 20 queries across exactly four categories. Distribute roughly evenly: ~5 queries per category.

#### Category definitions and what each query must look like

**Category A — `pure_recency`**: questions where the correct answer is the chronologically newest match. Time should dominate ranking. Examples:
- "what did we do in the last session"
- "last session for aibbe"
- "most recent bugfix"
- "what changes were made today"
- "latest decision about hsme"

For each `pure_recency` query, the **expected_winner_criterion** is a deterministic SQL predicate that picks exactly the row that SHOULD rank top-1. Example:
```yaml
expected_winner_criterion:
  description: "newest active session_summary across all projects"
  sql: "SELECT id FROM memories WHERE source_type='session_summary' AND status='active' AND superseded_by IS NULL ORDER BY created_at DESC, id DESC LIMIT 1"
```

**Category B — `pure_relevance`**: questions where chronology is irrelevant; the correct answer is the most semantically/lexically related memory regardless of age. The whole point is to stress-test that the eventual time-decay does NOT bury these. Examples:
- "how is FTS5 sanitization implemented"
- "what is the architecture of the hybrid search"
- "how does sqlite-vec store embeddings"
- "what does ComputeHash do"

For each `pure_relevance` query, the **expected_winner_criterion** can be a manual annotation referencing a specific memory id that you confirmed by running `mcp__hsme__search_fuzzy` against the current corpus. Document why that id is the "right" answer in a `notes` field.

**Category C — `mixed`**: questions where both relevance and recency matter. The "correct" answer is the most recent memory among those that are also semantically relevant. Examples:
- "what was the last bugfix to the search code"
- "recent decisions about session summaries"
- "latest changes to the schema"

**Category D — `adversarial`**: questions deliberately constructed so a naïve aggressive time-decay would give the WRONG answer. They test that decay doesn't make ancient-but-vital memories unreachable. Examples:
- "what is the original HSME architecture overview" — the answer is one of the earliest memories in the corpus (2026-04-03 era), and it MUST still be findable.
- "the v1.0.1 final architecture" — historical document, must not be buried.

For adversarial queries, the **expected_winner_criterion** identifies a specific old memory id and includes an `age_days_at_test_time` field documenting how old it is right now.

### Required YAML structure

```yaml
# docs/future-missions/mission-3-eval-set.yaml
schema_version: 1
frozen_at: "2026-04-26T<HH:MM:SS>Z"   # ISO timestamp of when you finalized it
total_queries: 20                      # or more, with ~5 per category
corpus_snapshot:
  hsme_db: /home/gary/dev/hsme/data/engram.db
  total_memories: <run COUNT(*) FROM memories at freeze time>
  date_range:
    min_created_at: <run MIN(created_at)>
    max_created_at: <run MAX(created_at)>

queries:
  - id: rec-01
    category: pure_recency
    query: "what did we do in the last session"
    expected_winner_criterion:
      description: "newest active session_summary across all projects"
      sql: "SELECT id FROM memories WHERE source_type='session_summary' AND status='active' AND superseded_by IS NULL ORDER BY created_at DESC, id DESC LIMIT 1"
    notes: ""
  - id: rec-02
    category: pure_recency
    query: "last session aibbe"
    expected_winner_criterion:
      description: "newest active session_summary for project=aibbe"
      sql: "SELECT id FROM memories WHERE source_type='session_summary' AND status='active' AND superseded_by IS NULL AND project='aibbe' ORDER BY created_at DESC, id DESC LIMIT 1"
    notes: ""
  # ... 18+ more, covering rel-XX, mix-XX, adv-XX
```

### Validation rules for the YAML

- Every query has a unique `id` of the form `<category-prefix>-<NN>`: `rec-`, `rel-`, `mix-`, `adv-`.
- Every query has a non-empty `query` string and a non-empty `expected_winner_criterion`.
- Every category has at least 4 queries.
- The YAML parses with a standard YAML loader (don't trust by eye — actually parse it before committing).
- Don't include any query that depends on memories that may be added or modified after this freeze. Freeze means freeze.

---

## Step 2 — Run the baseline measurement

For each query in the eval set, run `mcp__hsme__search_fuzzy` against the current corpus with `limit=10` and record where the expected winner appears in the result list.

### How to invoke search

The HSME MCP server exposes the relevant tools. From this session you can call them via the MCP tools (already wired in the harness):
- `mcp__hsme__search_fuzzy(query, limit=10)` — hybrid lexical+semantic with RRF fusion (no time decay yet — that's Mission 3's job).
- `mcp__hsme__search_exact(keyword, limit=10)` — FTS5 lexical only.

For each query, run `mcp__hsme__search_fuzzy` with `limit=10`. Capture:
- Raw result list (memory IDs in order).
- Rank (1-based) of the `expected_winner` (look it up by running the `expected_winner_criterion.sql` against the DB).
- If the expected winner is NOT in the top 10, record `rank: null` and `in_top_10: false`.

### Files to produce

#### `docs/future-missions/mission-3-baseline.json`

Machine-readable. Schema:

```json
{
  "schema_version": 1,
  "measured_at": "2026-04-26T<HH:MM:SS>Z",
  "tool_under_test": "search_fuzzy",
  "tool_version": "<commit SHA of HEAD at measurement time>",
  "total_queries": 20,
  "corpus_snapshot": {
    "total_memories": 1004,
    "min_created_at": "...",
    "max_created_at": "..."
  },
  "results": [
    {
      "id": "rec-01",
      "category": "pure_recency",
      "query": "what did we do in the last session",
      "expected_winner_id": 994,
      "actual_top_10_ids": [994, 987, ...],
      "expected_winner_rank": 1,
      "in_top_10": true,
      "in_top_3": true,
      "in_top_1": true
    }
    // ... one entry per query
  ],
  "summary": {
    "by_category": {
      "pure_recency":   { "n": 5, "top1_hit_rate": 0.4, "top3_hit_rate": 0.6, "top10_hit_rate": 1.0 },
      "pure_relevance": { "n": 5, "top1_hit_rate": 0.6, "top3_hit_rate": 0.8, "top10_hit_rate": 1.0 },
      "mixed":          { "n": 5, "top1_hit_rate": 0.2, "top3_hit_rate": 0.4, "top10_hit_rate": 0.8 },
      "adversarial":    { "n": 5, "top1_hit_rate": 0.4, "top3_hit_rate": 0.6, "top10_hit_rate": 1.0 }
    },
    "overall": {
      "top1_hit_rate": 0.4, "top3_hit_rate": 0.6, "top10_hit_rate": 0.95
    }
  }
}
```

#### `docs/future-missions/mission-3-baseline.md`

Human-readable companion. Should be ≤2 pages. Include:

1. **Header**: date measured, commit SHA, corpus size.
2. **Summary table**: hit-rates per category as a markdown table.
3. **Headline finding** in plain language (e.g., "pure_recency queries hit top-1 only 40% of the time today — this is the gap Mission 3 must close").
4. **Per-query result table**: `id | category | rank | top-1 hit | top-3 hit`.
5. **Notes on outliers**: any query that returned `null` rank, or any surprise.

This MD file is what humans will read; the JSON is for the eventual A/B harness in Mission 3.

---

## Step 3 — Persist and commit

Three files committed in a single commit on `main`:
- `docs/future-missions/mission-3-eval-set.yaml`
- `docs/future-missions/mission-3-baseline.json`
- `docs/future-missions/mission-3-baseline.md`

Suggested commit message (conventional commits, no AI attribution per repo policy):

```
docs(mission-3): freeze eval set + baseline measurements for RRF time-decay

- Adds 20+ queries across pure_recency, pure_relevance, mixed, adversarial.
- Captures search_fuzzy hit-rates per category against the current corpus.
- Pre-flight items 3 and 4 of docs/future-missions/mission-3-rrf-time-decay.md.
- Required reading before /spec-kitty.specify for Mission 3.
```

---

## Step 4 — Final verification before declaring done

Run this checklist mentally before committing. If any item fails, fix it.

- [ ] `mission-3-eval-set.yaml` has at least 20 queries with at least 4 per category.
- [ ] All 20 queries have a non-empty `expected_winner_criterion`.
- [ ] The YAML file parses cleanly (no syntax errors).
- [ ] `mission-3-baseline.json` has one `results` entry per query in the eval set (same `id`s).
- [ ] Each `results` entry has `actual_top_10_ids` populated (10 ids unless the corpus has fewer matching results).
- [ ] `summary.by_category` numbers add up consistently with the per-query data.
- [ ] `mission-3-baseline.md` is human-readable, ≤2 pages, and includes the headline finding.
- [ ] All three files are committed in a single commit on branch `main`.
- [ ] You did NOT run `/spec-kitty.specify`. The next operator does that.
- [ ] You did NOT modify any source file outside `docs/future-missions/`.

---

## Hand-off — what the NEXT operator does after this brief is done

When this brief is complete, the next operator runs:

```
/spec-kitty.specify sigue con la siguiente mision en docs/future-missions/README.md
```

During the discovery interview for Mission 3, they will reference `mission-3-eval-set.yaml` and `mission-3-baseline.json` to:
- Set NFR-002's threshold ("X% of pure_recency queries hit top-3 with decay on") relative to today's measured rate. The "after" target should be a concrete improvement: e.g., if today is 60%, target ≥85% with decay on.
- Set NFR-003's threshold ("Y% of adversarial queries still hit top-5 with decay on") to ensure decay doesn't bury vital old memories. The "after" target should be ≥ today's rate; ideally equal.
- Set NFR-001's overhead budget (≤5% latency) — this brief doesn't measure latency directly, but the eval set is the input to the future A/B harness.

**Do not modify this brief once it's committed.** It is the freeze point.

---

## Reference material (for the executing agent's context)

If you need more context while doing this work:

- `docs/future-missions/mission-3-rrf-time-decay.md` — the original draft for Mission 3.
- `kitty-specs/recency-fast-path-for-session-recall-01KQ405N/spec.md` — Mission 2's spec, sets the precedent for what "session summary" memories look like.
- `kitty-specs/engram-legacy-cutover-and-corpus-restoration-01KQ2SJK/spec.md` — Mission 1's spec, why `created_at` and `project` are reliable now.
- `ideas/session-history-recency.md` — the original idea that motivates the entire recency stack (Missions 2 and 3).
- `src/core/search/fuzzy.go` — the current `FuzzySearch` (no time decay). Read-only reference; do not modify.

If you have questions while executing — for example, you find that an `expected_winner_criterion` returns more than one row, or you discover the corpus has changed in a way that invalidates the eval set design — STOP and surface the question to the human operator instead of guessing. The whole point of this brief is to produce defensible numbers, not fast numbers.
