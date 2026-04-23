---
work_package_id: WP01
title: Fix DB Initialization
dependencies: []
requirement_refs:
- C-001
- FR-001
planning_base_branch: master
merge_target_branch: master
branch_strategy: 'Current branch at workflow start: master. Planning/base branch for this feature: master. Completed changes must merge into master.'
subtasks:
- T001
- T002
agent: claude
history: []
agent_profile: implementer-ivan
authoritative_surface: src/storage/sqlite/
execution_mode: code_change
owned_files:
- src/storage/sqlite/db.go
- src/core/indexer/ingest.go
role: implementer
tags: []
---

## ⚡ Do This First: Load Agent Profile
```bash
spec-kitty agent profile load --id implementer-ivan
```

## Objective
Remove the implicit FTS5 SQLite triggers that violate the system specification and ensure `ingest.go` correctly handles the explicit synchronization.

## Branch Strategy
Current branch at workflow start: master. Planning/base branch for this feature: master. Completed changes must merge into master.

## Subtasks

### T001: Remove implicit FTS5 triggers from SQLite schema
**Purpose**: Adhere to the design prohibiting triggers for external-content FTS5 tables.
**Steps**:
1. Open `src/storage/sqlite/db.go`.
2. Remove the `CREATE TRIGGER ... memory_chunks_ai`, `memory_chunks_ad`, and `memory_chunks_au` statements from the `schema` string.

### T002: Verify `ingest.go` explicitly syncs FTS5
**Purpose**: Ensure the application layer takes responsibility for the index.
**Steps**:
1. Open `src/core/indexer/ingest.go`.
2. Verify that the `StoreContext` function explicitly executes an `INSERT INTO memory_chunks_fts` command within its database transaction when adding new chunks.
