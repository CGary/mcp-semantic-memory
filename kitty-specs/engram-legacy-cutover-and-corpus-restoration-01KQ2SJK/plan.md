# Implementation Plan: Engram Legacy Cutover & Corpus Restoration

**Branch**: `main` | **Date**: 2026-04-25 | **Spec**: [spec.md](spec.md)
**Mission**: `engram-legacy-cutover-and-corpus-restoration-01KQ2SJK`
**Mission ID**: `01KQ2SJK44AP2YDKXSBKPZCB8Q`

---

## Summary

Restore lost chronology and project metadata for 842 migrated memories by joining HSME's wrapped `raw_content` against the legacy Engram observations DB on byte-equal content. Re-tag 62 fabricated session summaries to `source_type='session_summary'`. Delete 1 malformed empty memory. Ingest 59 post-migration orphan observations into HSME via the existing `indexer.StoreContext` path. Add a `project` column to `memories` and expose an optional `project` filter on `search_fuzzy` and `search_exact`. Cut Claude Code's MCP configuration to remove `engram` so HSME becomes the single write target. The operation is delivered as a new Go binary under `cmd/migrate-legacy/`, runs as one transaction per phase, takes a hot backup before any mutation, and produces an idempotent run report.

## Technical Context

| Item | Value |
|------|-------|
| **Language/Version** | Go (matches the rest of the repo: `cmd/hsme`, `cmd/worker`, `cmd/ops`). Module pinned to repo's existing `go.mod`. |
| **Primary Dependencies** | `mattn/go-sqlite3` with build tags `sqlite_fts5 sqlite_vec` (existing). Standard library only for the migrator core (no new direct dependencies). Reuses internal packages: `src/core/indexer` (StoreContext, hashing), `src/storage/sqlite` (init, migrations), `src/core/inference/ollama` (only when ingesting orphans, since they need fresh embeddings). |
| **Storage** | Two SQLite DBs: HSME at `/home/gary/dev/hsme/data/engram.db` (read-write), legacy Engram at `/home/gary/.engram/engram.db` (read-only, opened with `?mode=ro&immutable=1`). |
| **Testing** | Go tests under `tests/modules/` with `-tags "sqlite_fts5 sqlite_vec"`. Existing pattern: temp DBs initialized via `sqlite.InitDB`, then driver functions exercised. Reused for migrator tests. |
| **Target Platform** | Linux (CachyOS / Arch). Single-user developer machine; no multi-host coordination. |
| **Project Type** | Single project, monolithic Go module with multiple `cmd/` binaries. |
| **Performance Goals** | Backfill ≤ 10 minutes for the 905+59 corpus on the developer's machine (NFR-002). Search latency unchanged ±10% post schema change (NFR-004). Orphan ingestion bound by Ollama throughput (~1.8s/task observed on this machine, so 59 orphans ≈ 2 minutes worst case). |
| **Constraints** | Hot backup mandatory pre-mutation. One transaction per phase. Idempotent re-run. Legacy DB never modified. `raw_content` of matched rows never rewritten (preserves chunks, FTS, embeddings). |
| **Scale/Scope** | 905 migrated rows + 59 orphans + 1 garbage row = 965 affected memories. Single user, single host. |

## Charter Check

**SKIPPED** — `.kittify/charter/charter.md` not found at planning time. No project charter exists to gate against. Standard engineering practice applies: tests, idempotency, backup before destructive action, no silent data loss.

## Project Structure

### Documentation (this feature)

```
kitty-specs/engram-legacy-cutover-and-corpus-restoration-01KQ2SJK/
├── spec.md              # Feature specification
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output (operator runbook)
├── contracts/           # Phase 1 output
│   ├── migrator-cli.md       # cmd/migrate-legacy CLI contract
│   ├── search-tools.md       # MCP search_fuzzy / search_exact updated contracts
│   └── verify-cutover.md     # scripts/verify_cutover.sh contract
├── checklists/
│   └── requirements.md  # From specify
├── meta.json
└── tasks/               # Created by /spec-kitty.tasks
```

### Source Code (repository root)

```
cmd/
├── hsme/                # MCP server (existing, unchanged)
├── worker/              # Async task worker (existing, unchanged)
├── ops/                 # Observability ops (existing, unchanged)
└── migrate-legacy/      # NEW — one-shot migration binary
    ├── main.go              # CLI entry, flag parser, dispatcher
    ├── phases.go            # Phase 1..7 orchestration
    ├── matcher.go           # Wrapper-format parser + content equality match
    ├── report.go            # JSON + text run report writers
    ├── backup.go            # Wraps scripts/backup_hot.sh invocation
    └── main_test.go         # Table-driven phase tests with temp DBs

src/
├── core/
│   ├── indexer/         # ingest.go reused as-is for orphan ingestion
│   └── search/
│       ├── fuzzy.go         # MODIFIED — add optional project filter
│       └── graph.go         # unchanged
├── mcp/
│   └── handler.go       # MODIFIED — surface project param on search_fuzzy / search_exact
└── storage/
    └── sqlite/
        └── db.go        # MODIFIED — add migration adding `project` column + index

scripts/
├── backup_hot.sh        # existing, unchanged (invoked by migrator)
├── restore.sh           # existing, unchanged
└── verify_cutover.sh    # NEW — telemetry for NFR-005 (rowcount + filesize + max(created_at) snapshot of legacy DB)

tests/
└── modules/
    ├── migrate_legacy_test.go   # NEW — tests for matcher, phases, idempotency
    ├── search_test.go           # MODIFIED — exercise project filter
    └── ...                      # existing tests unchanged

data/
└── migrations/                  # NEW — migrator output, gitignored
    └── <run_id>/
        ├── report.json
        ├── report.txt
        ├── mappings.tsv
        └── unmatched.tsv
```

**Rationale for `cmd/migrate-legacy/`**: matches the existing convention of one binary per `cmd/` subfolder. One-shot or not, a Go binary is testable, idempotent by construction, replaces the prompt-driven migration that lost data the first time. This is exactly the kind of operation where "do it once, verify, never again" is the goal — and that warrants real code, not a script with `sqlite3` heredocs.

## Phase Definition

The migration runs in 7 phases, executed in strict order by `cmd/migrate-legacy`:

| # | Phase | Action | Mutates HSME? | Reads Legacy? |
|---|-------|--------|---------------|---------------|
| 0 | **Preflight** | Verify schema, both DBs reachable, sufficient disk for backup. Refuse to proceed on any failure. | No | Yes (read-only) |
| 1 | **Backup** | Invoke `scripts/backup_hot.sh`. Refuse to proceed if backup fails or backup file size is suspicious. | No | No |
| 2 | **Schema migration** | `ALTER TABLE memories ADD COLUMN project TEXT;` + `CREATE INDEX idx_memories_project ON memories(project);` Wrapped in tx. Idempotent via existence check on `pragma_table_info`. | Yes (schema only) | No |
| 3 | **Backfill matched** | For each row with `source_type IN ('engram_migration','engram_session_migration')`: parse wrapper, look up legacy by content equality, UPDATE `created_at`/`source_type`/`project`. Single tx. ~842 rows expected. | Yes | Yes |
| 4 | **Retag born-in-HSME** | For rows still tagged `engram_session_migration` after phase 3 (the 62 fabricated summaries): UPDATE `source_type='session_summary'` + backfill `project` from the wrapper. Single tx. | Yes | No |
| 5 | **Delete garbage** | DELETE the malformed empty row(s) (HSME id 722 OR any row matching the verified rule: `source_type LIKE 'engram%' AND length(raw_content) < 50 AND raw_content NOT LIKE 'Title: %\n\n%'`). Cascade triggers handle chunks + FTS. | Yes | No |
| 6 | **Pre-cutover snapshot** | Read `MAX(created_at)`, `COUNT(*)`, and filesize of legacy. Write to run report. This becomes the "before MCP reconfig" baseline. | No | Yes (read-only) |
| 7 | **Ingest orphans** | For each legacy observation NOT matched in phase 3 (the 59 orphans): build the wrapper format, call `indexer.StoreContext` with the legacy `created_at` injected. Phase runs to completion regardless of Ollama latency — graph-extraction tasks land in the async queue and process after. | Yes | Yes |

**Cutover step (manual, runbook)**: After phase 7 completes, the operator runs `claude mcp remove engram` and reloads Claude Code. Then runs the migrator a second time in `--mode=delta` to pick up any race-window writes. Then runs `scripts/verify_cutover.sh` at T0 and again at T+24h to verify NFR-005.

The race-window handling (Option A from planning) is implemented as: phase 6 captures pre-cutover state; the operator does the MCP reconfig manually; the operator re-invokes `cmd/migrate-legacy --mode=delta` which runs only phase 7 with a filter `WHERE created_at > <pre_snapshot_max>`.

## Phase 0: Outline & Research → research.md

See [research.md](research.md) for full decisions and alternatives. Topics covered:

- Why content-equality match instead of hash match (algorithm divergence)
- Why ALTER TABLE ADD COLUMN is safe (SQLite idempotency, no rewrite)
- Why per-phase transaction is correct (atomicity per logical unit, simpler resumption)
- Why we never rewrite `raw_content` of matched rows (chunk/FTS/embedding integrity)
- Why orphans go through `StoreContext` instead of raw INSERT (chunks, FTS triggers, async tasks)
- Why fail-loud cutover (Option A) over zero-downtime (Option B)
- Why no automatic MCP reconfig (config change belongs to user, not migrator)

## Phase 1: Design & Contracts → data-model.md, contracts/, quickstart.md

- **[data-model.md](data-model.md)**: HSME `memories` schema after migration, legacy `observations` schema as read source, run report structure, wrapper format grammar.
- **[contracts/migrator-cli.md](contracts/migrator-cli.md)**: CLI surface, flags, modes (`full`, `delta`, `dry-run`), exit codes.
- **[contracts/search-tools.md](contracts/search-tools.md)**: Updated `search_fuzzy` / `search_exact` MCP signatures with optional `project` parameter.
- **[contracts/verify-cutover.md](contracts/verify-cutover.md)**: `scripts/verify_cutover.sh` interface and output format.
- **[quickstart.md](quickstart.md)**: End-to-end operator runbook including the cutover sequence, T+24h verification, and rollback procedure.

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Legacy match-rate drops below 93% at execution time | Low | Medium | Migrator reports unmatched count; threshold check refuses to apply phase 4 if unmatched ratio > 10% (configurable). Operator reviews before proceeding. |
| Schema migration fails mid-run | Very Low | High | Pre-mutation hot backup (phase 1). Hot backup uses SQLite's atomic backup API; restore via `scripts/restore.sh`. |
| Phase 3 transaction conflicts with concurrent MCP writes | Low | Medium | MCP server is read-mostly. SQLite WAL allows concurrent reads. On write conflict, transaction retries with exponential backoff (3 attempts). |
| Ollama is down/slow during phase 7 | Medium | Low | `StoreContext` enqueues graph-extraction asynchronously; embedding has a 10s context timeout. Phase 7 records synchronous success; async tasks complete later via the worker. |
| User forgets to run the T+24h verification | Medium | Low (just unverified, not broken) | `quickstart.md` schedules a follow-up; consider a `/schedule` background agent for the 24h check. |
| Race window write between phase 6 and `claude mcp remove engram` | Medium | Low | `--mode=delta` second invocation catches any new rows. Cutover is fail-loud per Option A — write attempts during the gap fail visibly because engram MCP is gone. |

## Definition of Done

1. All 12 functional requirements (FR-001..FR-012) verified against the live DB.
2. All 6 NFRs measured and within thresholds, with measurements recorded in the run report.
3. All 7 constraints honored (verifiable from run report + code review).
4. Migrator binary committed with green tests under `tests/modules/migrate_legacy_test.go`.
5. `scripts/verify_cutover.sh` committed with a 0-exit-code dry-run on a clean state.
6. T+24h verification report recorded (separately from this PR — operator submits it as a comment on the merged PR or a follow-up note).
7. Documentation updates: README mentions the new binary and the `project` column.
8. Branch `main` (current) — direct landing per branch contract.

## Branch Contract Reaffirmation

- **Current branch:** `main`
- **Planning/base branch:** `main`
- **Final merge target:** `main`
- **`branch_matches_target`:** `true`

Direct land on `main` per the deterministic branch contract. The implementation phase may use a worktree branch per spec-kitty's worktree convention.

---

## Next Step

Run `/spec-kitty.tasks` to break this plan into work packages. Do NOT proceed to implementation from this command.
