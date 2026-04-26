# Future Mission 2 — Recency-Aware Fast Path

**Status**: Drafted, NOT YET CREATED in spec-kitty.
**Depends on**: Mission 1 (`engram-legacy-cutover-and-corpus-restoration-01KQ2SJK`) merged.
**Reason for deferral**: The thresholds of this mission depend on the baseline measured AFTER Mission 1 restores real `created_at` values. Specifying it now would lock in numbers based on broken data.

## Purpose

Solve the recency gap in `search_fuzzy`: today, semantic search prioritizes relevance over chronology, so when an agent asks "what did we do in the last session?" it gets results from three weeks ago ranked higher than the actual latest session. This mission introduces a cheap, lexical fast path for the recency case without touching the RRF engine (that's Mission 3).

## Source idea

`ideas/session-history-recency.md` — Solution 1 ("Session-Tagged Summaries") and the doctrinal piece of Solution 3 ("Contextual Anchoring", documented as client guidance, not server feature).

## Scope

### In scope

1. **Convention**: Standardize `source_type='session_summary'` as the canonical type for any agent-written session recap. Document the contract in `CLAUDE.md` and the HSME README.
2. **New MCP tool**: `recall_recent_session(project?, limit=5)` returns the latest N memories with `source_type='session_summary'` ordered by `created_at DESC`. No embedder involved — pure SQL + index lookup.
3. **Filter passthrough**: Reuse the `project` column added in Mission 1 so the tool can scope to a single project.
4. **Documentation**: Update `CLAUDE.md` HSME protocol section to instruct agents to call `recall_recent_session` BEFORE `search_fuzzy` when the question is "what was the last X".

### Out of scope (deferred to Mission 3)

- RRF time-decay in the search ranking algorithm.
- Changes to `search_fuzzy` or `search_exact` ranking.
- Session-stack/contextual-anchoring inside the MCP server (this is a client-side concern; only the protocol guidance lives here).

## Pre-flight checks before opening this mission

1. Confirm Mission 1 merged and `created_at` values look correct on a sample query.
2. Confirm the `project` column is populated for ≥99% of session summaries.
3. Run a baseline query: `search_fuzzy("what did we do last")` and record where the most recent session summary appears in the ranking. This becomes the "before" number for Mission 2's success criteria.

## Likely Functional Requirements (preview, not authoritative)

- FR-001: New MCP tool `recall_recent_session` with optional `project` and `limit`.
- FR-002: The tool MUST NOT call the embedder (latency budget < 100ms P50).
- FR-003: An index on `(source_type, created_at DESC)` SHOULD exist if not already implied by other indexes.
- FR-004: `CLAUDE.md` HSME protocol section MUST direct agents to use the new tool for "last session" style queries.
- FR-005: Existing search tools remain unchanged in behavior.

## Likely Non-Functional Requirements (preview)

- NFR-001: P50 latency for `recall_recent_session` ≤ 100ms.
- NFR-002: Result correctness on a fixture corpus of 50 session summaries: top-1 result matches the chronologically newest session 100% of the time.

## Open questions to resolve at specify time

- Q1: Should `recall_recent_session` filter out superseded memories? (Probably yes — superseded summaries are obsolete by definition.)
- Q2: Should the limit have a hard upper bound to prevent the agent from pulling 1000 summaries at once? (Yes — suggest cap at 50.)
- Q3: Should the convention also enforce a Session ID format inside the content? (Cheap to add, pays off for cross-referencing sessions to other artifacts.)

## Effort estimate

Small. ~3-4h:
- 1h SQL + tool wiring.
- 1h tests.
- 30min docs / CLAUDE.md update.
- 1h end-to-end verification with a populated DB.

## Why this is not Mission 1

The recency fast path is useless if `created_at` is fake. Mission 1 must complete first.

## Why this is not folded into Mission 3

Mission 3 (RRF time-decay) touches the ranking core of `search_fuzzy`. Mixing a low-risk new tool with a high-risk core change in one PR turns a quick win into a risky change. Keeping them split lets Mission 2 ship in a day and Mission 3 take its full verify cycle without holding back the easy gain.
