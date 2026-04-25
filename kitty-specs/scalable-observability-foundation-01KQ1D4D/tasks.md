# Work Packages: Scalable Observability Foundation

**Mission**: `scalable-observability-foundation-01KQ1D4D`
**Branch**: `main`

## Subtask Index
| ID | Description | WP | Parallel |
|---|---|---|---|
| T001 | Extend SQLite schema with observability tables, views, default policies, and rollup job seeds | WP01 | |
| T002 | Add storage-level helpers for observability configuration, policy reads, and transactional write support | WP01 | |
| T003 | Introduce observability config loading and runtime level/sampling decisions | WP02 | [P] |
| T004 | Implement trace/span/event models plus a SQLite-backed recorder core | WP02 | |
| T005 | Implement rollup and retention service primitives with checkpoint-aware interfaces | WP02 | |
| T006 | Integrate recorder lifecycle into MCP request handling stages | WP03 | |
| T007 | Wire observability configuration and recorder bootstrap into the MCP binary | WP03 | |
| T008 | Integrate recorder lifecycle into worker leasing and task execution stages | WP04 | |
| T009 | Wire observability configuration and recorder bootstrap into the worker binary | WP04 | |
| T010 | Add a dedicated operations runner binary for rollups, retention, and housekeeping loops | WP05 | |
| T011 | Implement maintenance execution flow that emits self-observability traces and advances checkpoints | WP05 | |
| T012 | Expose operator-oriented query helpers/commands and update local operational workflow docs/build wiring | WP05 | |

## WP01: Schema & SQLite Foundation
**Goal**: Extend the SQLite layer with the exact observability schema, seeded policies, views, and storage primitives needed by the rest of the system.
**Prompt**: `tasks/WP01-schema-sqlite-foundation.md` (~300 lines)
**Dependencies**: None
**Included Subtasks**:
- [ ] T001 Extend SQLite schema with observability tables, views, default policies, and rollup job seeds (WP01)
- [ ] T002 Add storage-level helpers for observability configuration, policy reads, and transactional write support (WP01)

## WP02: Recorder Core & Maintenance Services
**Goal**: Create the reusable observability package that models traces/spans/events, loads config, persists telemetry, and provides rollup/retention service primitives.
**Prompt**: `tasks/WP02-recorder-core-maintenance.md` (~420 lines)
**Dependencies**: WP01
**Included Subtasks**:
- [ ] T003 Introduce observability config loading and runtime level/sampling decisions (WP02)
- [ ] T004 Implement trace/span/event models plus a SQLite-backed recorder core (WP02)
- [ ] T005 Implement rollup and retention service primitives with checkpoint-aware interfaces (WP02)

## WP03: MCP Runtime Capture
**Goal**: Instrument the MCP server and main binary so request lifecycle stages persist observability data under the new recorder contract.
**Prompt**: `tasks/WP03-mcp-runtime-capture.md` (~320 lines)
**Dependencies**: WP01, WP02
**Included Subtasks**:
- [ ] T006 Integrate recorder lifecycle into MCP request handling stages (WP03)
- [ ] T007 Wire observability configuration and recorder bootstrap into the MCP binary (WP03)

## WP04: Worker Runtime Capture
**Goal**: Instrument the async semantic worker so leasing, execution, failures, and dependency calls emit correlated observability traces.
**Prompt**: `tasks/WP04-worker-runtime-capture.md` (~320 lines)
**Dependencies**: WP01, WP02
**Parallel Opportunities**: Can proceed in parallel with WP03 after recorder primitives exist.
**Included Subtasks**:
- [ ] T008 Integrate recorder lifecycle into worker leasing and task execution stages (WP04)
- [ ] T009 Wire observability configuration and recorder bootstrap into the worker binary (WP04)

## WP05: Operations Runner & Operator Surfaces
**Goal**: Introduce the dedicated ops runner for rollups/retention and provide operator-oriented query/usage surfaces without executing review or test workflows yet.
**Prompt**: `tasks/WP05-ops-runner-operator-surfaces.md` (~380 lines)
**Dependencies**: WP01, WP02, WP03, WP04
**Included Subtasks**:
- [ ] T010 Add a dedicated operations runner binary for rollups, retention, and housekeeping loops (WP05)
- [ ] T011 Implement maintenance execution flow that emits self-observability traces and advances checkpoints (WP05)
- [ ] T012 Expose operator-oriented query helpers/commands and update local operational workflow docs/build wiring (WP05)
